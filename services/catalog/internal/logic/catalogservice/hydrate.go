package catalogservicelogic

import (
	"context"

	"github.com/menli02/QR-menu/proto/catalog/v1"
	"github.com/menli02/QR-menu/services/catalog/internal/model"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

// hydrateItems batches the modifier-group/option/availability-window
// lookups for a page of items and assembles each into a wire MenuItem —
// one query per level (groups for all items, options for all those
// groups, windows for all items), not one per item, however many items
// are on the page. Shared by GetMenu, ListMenuItems, and every
// single-item mutation response (Create/Update/SetAvailability all
// return the full MenuItem).
func hydrateItems(ctx context.Context, conn sqlx.Session, items []model.MenuItem, currency string) ([]*v1_catalogpb.MenuItem, error) {
	if len(items) == 0 {
		return nil, nil
	}

	itemIDs := make([]string, len(items))
	for i, it := range items {
		itemIDs[i] = it.ID
	}

	groups, err := model.NewModifierGroupModel(conn).ListByItemIDs(ctx, itemIDs)
	if err != nil {
		return nil, err
	}
	groupIDs := make([]string, len(groups))
	groupsByItem := map[string][]model.ModifierGroup{}
	for i, g := range groups {
		groupIDs[i] = g.ID
		groupsByItem[g.ItemID] = append(groupsByItem[g.ItemID], groups[i])
	}

	options, err := model.NewModifierOptionModel(conn).ListByGroupIDs(ctx, groupIDs)
	if err != nil {
		return nil, err
	}
	optionsByGroup := map[string][]model.ModifierOption{}
	for i, o := range options {
		optionsByGroup[o.GroupID] = append(optionsByGroup[o.GroupID], options[i])
	}

	windows, err := model.NewAvailabilityWindowModel(conn).ListByItemIDs(ctx, itemIDs)
	if err != nil {
		return nil, err
	}
	windowsByItem := map[string][]model.AvailabilityWindow{}
	for i, w := range windows {
		windowsByItem[w.ItemID] = append(windowsByItem[w.ItemID], windows[i])
	}

	out := make([]*v1_catalogpb.MenuItem, len(items))
	for i := range items {
		out[i] = itemToProto(&items[i], currency, groupsByItem[items[i].ID], optionsByGroup, windowsByItem[items[i].ID])
	}
	return out, nil
}

// hydrateItem is hydrateItems for exactly one row — every single-item
// mutation response needs this.
func hydrateItem(ctx context.Context, conn sqlx.Session, item *model.MenuItem, currency string) (*v1_catalogpb.MenuItem, error) {
	items, err := hydrateItems(ctx, conn, []model.MenuItem{*item}, currency)
	if err != nil {
		return nil, err
	}
	return items[0], nil
}
