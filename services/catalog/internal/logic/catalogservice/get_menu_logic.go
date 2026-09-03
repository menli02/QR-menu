package catalogservicelogic

import (
	"context"
	"errors"
	"strconv"
	"time"

	"github.com/menli02/QR-menu/proto/catalog/v1"
	"github.com/menli02/QR-menu/services/catalog/internal/model"
	"github.com/menli02/QR-menu/services/catalog/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type GetMenuLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetMenuLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetMenuLogic {
	return &GetMenuLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// GetMenu returns the full active, visible, in-locale... in practice
// multi-locale (FR-C5): Category/MenuItem.name are LocalizedText — the
// full translation map, not a single resolved string — so `locale` here
// doesn't select a language server-side; the caller picks which
// translation to render. (Contrast ResolveOrderItems, which snapshots a
// single resolved string per FR-O3 — it builds an immutable order
// record, not a display payload.)
func (l *GetMenuLogic) GetMenu(in *v1_catalogpb.GetMenuRequest) (*v1_catalogpb.GetMenuResponse, error) {
	venue, err := model.NewVenueModel(l.svcCtx.DB).FindByID(l.ctx, in.GetVenueId())
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			return nil, status.Error(codes.NotFound, "venue not found")
		}
		return nil, status.Errorf(codes.Internal, "look up venue: %v", err)
	}

	menuVersion := strconv.FormatInt(venue.MenuVersion, 10)
	if req := in.GetIfMenuVersion(); req != "" && req == menuVersion {
		return &v1_catalogpb.GetMenuResponse{
			VenueId:     venue.ID,
			MenuVersion: menuVersion,
			Currency:    venue.Currency,
			NotModified: true,
		}, nil
	}

	categories, err := model.NewCategoryModel(l.svcCtx.DB).ListVisible(l.ctx, in.GetVenueId())
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list categories: %v", err)
	}

	items, err := model.NewMenuItemModel(l.svcCtx.DB).ListForMenu(l.ctx, in.GetVenueId())
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list menu items: %v", err)
	}

	itemIDs := make([]string, len(items))
	for i, it := range items {
		itemIDs[i] = it.ID
	}
	windows, err := model.NewAvailabilityWindowModel(l.svcCtx.DB).ListByItemIDs(l.ctx, itemIDs)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list availability windows: %v", err)
	}
	windowsByItem := map[string][]model.AvailabilityWindow{}
	for _, w := range windows {
		windowsByItem[w.ItemID] = append(windowsByItem[w.ItemID], w)
	}
	nowMinute := venueLocalMinuteOfDay(venue.Timezone)
	items = filterByAvailabilityWindow(items, windowsByItem, nowMinute)

	itemsByCategory := map[string][]model.MenuItem{}
	for _, it := range items {
		itemsByCategory[it.CategoryID] = append(itemsByCategory[it.CategoryID], it)
	}

	menuCategories := make([]*v1_catalogpb.MenuCategory, len(categories))
	for i := range categories {
		cat := &categories[i]
		protoItems, err := hydrateItems(l.ctx, l.svcCtx.DB, itemsByCategory[cat.ID], venue.Currency)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "load modifiers: %v", err)
		}
		menuCategories[i] = &v1_catalogpb.MenuCategory{
			Category: categoryToProto(cat),
			Items:    protoItems,
		}
	}

	return &v1_catalogpb.GetMenuResponse{
		VenueId:     venue.ID,
		MenuVersion: menuVersion,
		Currency:    venue.Currency,
		Categories:  menuCategories,
	}, nil
}

// venueLocalMinuteOfDay returns the current minute-of-day (0..1439) in
// tz, falling back to UTC if tz is unrecognized — a venue with a bad
// timezone value should degrade to "compare in UTC", not fail every
// guest menu request.
func venueLocalMinuteOfDay(tz string) int32 {
	loc, err := time.LoadLocation(tz)
	if err != nil {
		loc = time.UTC
	}
	now := time.Now().In(loc)
	return int32(now.Hour()*60 + now.Minute())
}

// filterByAvailabilityWindow implements FR-C7: an item with no windows
// defined is always shown; one with windows is shown only if nowMinute
// falls inside at least one of them. Windows are assumed same-day
// (start <= end, e.g. "08:00-11:00") — an overnight window like
// "22:00-02:00" isn't handled, since nothing in docs/TZ.md calls for one.
func filterByAvailabilityWindow(items []model.MenuItem, windowsByItem map[string][]model.AvailabilityWindow, nowMinute int32) []model.MenuItem {
	out := items[:0:0]
	for _, it := range items {
		windows := windowsByItem[it.ID]
		if len(windows) == 0 {
			out = append(out, it)
			continue
		}
		for _, w := range windows {
			if nowMinute >= w.StartMinuteOfDay && nowMinute <= w.EndMinuteOfDay {
				out = append(out, it)
				break
			}
		}
	}
	return out
}
