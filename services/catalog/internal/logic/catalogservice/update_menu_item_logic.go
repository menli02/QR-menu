package catalogservicelogic

import (
	"context"
	"errors"

	"github.com/menli02/QR-menu/proto/catalog/v1"
	"github.com/menli02/QR-menu/services/catalog/internal/model"
	"github.com/menli02/QR-menu/services/catalog/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type UpdateMenuItemLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewUpdateMenuItemLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateMenuItemLogic {
	return &UpdateMenuItemLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// UpdateMenuItem never touches is_active or is_available — those have
// their own narrower entry points (DeleteMenuItem, SetItemAvailability).
// It does replace the item's availability-window set wholesale (FR-C7):
// the wire message always carries the complete list.
func (l *UpdateMenuItemLogic) UpdateMenuItem(in *v1_catalogpb.UpdateMenuItemRequest) (*v1_catalogpb.MenuItem, error) {
	item := in.GetItem()
	if item == nil || len(item.GetName().GetTranslations()) == 0 || item.GetCategoryId() == "" {
		return nil, status.Error(codes.InvalidArgument, "item with a name and category_id is required")
	}
	if item.GetBasePrice().GetAmountMinor() < 0 {
		return nil, status.Error(codes.InvalidArgument, "base_price must not be negative")
	}

	venue, err := model.NewVenueModel(l.svcCtx.DB).FindByID(l.ctx, item.GetVenueId())
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			return nil, status.Error(codes.NotFound, "venue not found")
		}
		return nil, status.Errorf(codes.Internal, "look up venue: %v", err)
	}
	if c := item.GetBasePrice().GetCurrency(); c != "" && c != venue.Currency {
		return nil, status.Errorf(codes.InvalidArgument, "base_price currency %q does not match venue currency %q", c, venue.Currency)
	}

	windows := make([]model.AvailabilityWindow, len(item.GetAvailabilityWindows()))
	for i, w := range item.GetAvailabilityWindows() {
		windows[i] = model.AvailabilityWindow{
			ItemID:           item.GetId(),
			StartMinuteOfDay: w.GetStartMinuteOfDay(),
			EndMinuteOfDay:   w.GetEndMinuteOfDay(),
		}
	}

	var updated *model.MenuItem
	err = l.svcCtx.DB.TransactCtx(l.ctx, func(ctx context.Context, session sqlx.Session) error {
		itemModel := model.NewMenuItemModel(session)
		var err error
		updated, err = itemModel.Update(ctx, item.GetId(), item.GetVenueId(), item.GetCategoryId(),
			item.GetName().GetTranslations(), item.GetDescription().GetTranslations(),
			item.GetBasePrice().GetAmountMinor(), item.GetImageUrl(), item.GetAllergens(), item.GetSortOrder())
		if err != nil {
			return err
		}
		if err := model.NewAvailabilityWindowModel(session).ReplaceForItem(ctx, item.GetId(), windows); err != nil {
			return err
		}
		_, err = model.NewVenueModel(session).BumpMenuVersion(ctx, item.GetVenueId())
		return err
	})
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			return nil, status.Error(codes.NotFound, "menu item not found")
		}
		return nil, status.Errorf(codes.Internal, "update menu item: %v", err)
	}

	out, err := hydrateItem(l.ctx, l.svcCtx.DB, updated, venue.Currency)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "load modifiers: %v", err)
	}
	return out, nil
}
