package catalogservicelogic

import (
	"context"
	"errors"
	"strconv"

	"github.com/menli02/QR-menu/proto/catalog/v1"
	"github.com/menli02/QR-menu/services/catalog/internal/model"
	"github.com/menli02/QR-menu/services/catalog/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type ResolveOrderItemsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewResolveOrderItemsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ResolveOrderItemsLogic {
	return &ResolveOrderItemsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// ResolveOrderItems re-prices and validates a cart server-side (FR-O3):
// the client-displayed cart is never trusted. This is the only place in
// catalog that resolves LocalizedText down to a single string, because
// its output is a price+name *snapshot* for an immutable order record
// (order_db denormalizes it), not a display payload — contrast GetMenu,
// which returns the full translation map.
//
// Scope boundary: this validates item/modifier availability and
// computes prices only. FR-O5's other submit-time checks (qty 1..99,
// comment length, items-per-order, order total vs. venue limit) belong
// to the order service, which is what actually receives the submit and
// owns that request's full validation — duplicating a subset of them
// here would just be two sources of truth for the same rule.
func (l *ResolveOrderItemsLogic) ResolveOrderItems(in *v1_catalogpb.ResolveOrderItemsRequest) (*v1_catalogpb.ResolveOrderItemsResponse, error) {
	venue, err := model.NewVenueModel(l.svcCtx.DB).FindByID(l.ctx, in.GetVenueId())
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			return nil, status.Error(codes.NotFound, "venue not found")
		}
		return nil, status.Errorf(codes.Internal, "look up venue: %v", err)
	}
	locale := in.GetLocale()
	if locale == "" {
		locale = venue.DefaultLocale
	}

	itemIDs := make([]string, 0, len(in.GetItems()))
	seenItem := map[string]bool{}
	for _, ri := range in.GetItems() {
		if !seenItem[ri.GetItemId()] {
			seenItem[ri.GetItemId()] = true
			itemIDs = append(itemIDs, ri.GetItemId())
		}
	}

	items, err := model.NewMenuItemModel(l.svcCtx.DB).FindByIDs(l.ctx, in.GetVenueId(), itemIDs)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "look up items: %v", err)
	}
	itemByID := make(map[string]*model.MenuItem, len(items))
	for i := range items {
		itemByID[items[i].ID] = &items[i]
	}

	groups, err := model.NewModifierGroupModel(l.svcCtx.DB).ListByItemIDs(l.ctx, itemIDs)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "look up modifier groups: %v", err)
	}
	groupsByItem := map[string][]model.ModifierGroup{}
	groupIDs := make([]string, len(groups))
	for i, g := range groups {
		groupIDs[i] = g.ID
		groupsByItem[g.ItemID] = append(groupsByItem[g.ItemID], groups[i])
	}

	options, err := model.NewModifierOptionModel(l.svcCtx.DB).ListByGroupIDs(l.ctx, groupIDs)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "look up modifier options: %v", err)
	}
	optionByID := make(map[string]*model.ModifierOption, len(options))
	for i := range options {
		optionByID[options[i].ID] = &options[i]
	}

	resp := &v1_catalogpb.ResolveOrderItemsResponse{
		Currency:    venue.Currency,
		MenuVersion: strconv.FormatInt(venue.MenuVersion, 10),
	}

	for _, ri := range in.GetItems() {
		item, ok := itemByID[ri.GetItemId()]
		if !ok {
			resp.Unavailable = append(resp.Unavailable, &v1_catalogpb.UnavailableItem{
				ItemId: ri.GetItemId(), Reason: "NOT_FOUND",
			})
			continue
		}
		name := resolveLocale(item.Name, locale, venue.DefaultLocale)
		if !item.IsActive {
			resp.Unavailable = append(resp.Unavailable, &v1_catalogpb.UnavailableItem{
				ItemId: item.ID, Name: name, Reason: "INACTIVE",
			})
			continue
		}
		if !item.IsAvailable {
			resp.Unavailable = append(resp.Unavailable, &v1_catalogpb.UnavailableItem{
				ItemId: item.ID, Name: name, Reason: "OUT_OF_STOCK",
			})
			continue
		}

		resolvedMods, unitPriceDelta, invalidReason := resolveModifiers(item, ri.GetModifierOptionIds(), groupsByItem[item.ID], optionByID, locale, venue.DefaultLocale, venue.Currency)
		if invalidReason != "" {
			resp.Unavailable = append(resp.Unavailable, &v1_catalogpb.UnavailableItem{
				ItemId: item.ID, Name: name, Reason: invalidReason,
			})
			continue
		}

		qty := ri.GetQty()
		unitPriceMinor := item.BasePriceMinor + unitPriceDelta
		lineTotal := unitPriceMinor * int64(qty)

		resp.Items = append(resp.Items, &v1_catalogpb.ResolvedItem{
			ItemId:         item.ID,
			Name:           name,
			UnitPrice:      money(unitPriceMinor, venue.Currency),
			Modifiers:      resolvedMods,
			Qty:            qty,
			LineTotalMinor: lineTotal,
			Comment:        ri.GetComment(),
		})
		resp.TotalMinor += lineTotal
	}

	return resp, nil
}

// resolveModifiers validates requestedOptionIDs against item's modifier
// groups (every group's [min_select, max_select] and required flag —
// FR-C3) and, if valid, returns the resolved snapshot plus the total
// price delta. A non-empty reason means the selection is invalid and the
// item as a whole should be reported unavailable.
func resolveModifiers(item *model.MenuItem, requestedOptionIDs []string, groups []model.ModifierGroup, optionByID map[string]*model.ModifierOption, locale, defaultLocale, currency string) (resolved []*v1_catalogpb.ResolvedModifier, priceDelta int64, reason string) {
	requested := map[string]bool{}
	for _, id := range requestedOptionIDs {
		requested[id] = true
	}

	// Every requested option must actually belong to one of this item's
	// groups — an id from a different item (or that doesn't exist) is
	// tampering or a stale cart, not a legitimate selection.
	optionGroup := map[string]string{} // option id -> group id, restricted to this item's groups
	for _, g := range groups {
		for _, o := range optionByID {
			if o.GroupID == g.ID {
				optionGroup[o.ID] = g.ID
			}
		}
	}
	for id := range requested {
		if _, ok := optionGroup[id]; !ok {
			return nil, 0, "INVALID_MODIFIER_SELECTION"
		}
	}

	selectedByGroup := map[string]int{}
	for optID, groupID := range optionGroup {
		if requested[optID] {
			selectedByGroup[groupID]++
		}
	}
	for _, g := range groups {
		n := selectedByGroup[g.ID]
		if n < int(g.MinSelect) || n > int(g.MaxSelect) || (g.Required && n == 0) {
			return nil, 0, "INVALID_MODIFIER_SELECTION"
		}
	}

	for optID := range requested {
		opt := optionByID[optID]
		resolved = append(resolved, &v1_catalogpb.ResolvedModifier{
			OptionId:   opt.ID,
			Name:       resolveLocale(opt.Name, locale, defaultLocale),
			PriceDelta: money(opt.PriceDeltaMinor, currency),
		})
		priceDelta += opt.PriceDeltaMinor
	}
	return resolved, priceDelta, ""
}

// resolveLocale picks the best available translation: the requested
// locale, then the venue default, then "en", then whatever's there.
func resolveLocale(m map[string]string, locale, defaultLocale string) string {
	if v, ok := m[locale]; ok && v != "" {
		return v
	}
	if v, ok := m[defaultLocale]; ok && v != "" {
		return v
	}
	if v, ok := m["en"]; ok && v != "" {
		return v
	}
	for _, v := range m {
		if v != "" {
			return v
		}
	}
	return ""
}
