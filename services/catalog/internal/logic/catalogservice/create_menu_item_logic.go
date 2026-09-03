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

type CreateMenuItemLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCreateMenuItemLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateMenuItemLogic {
	return &CreateMenuItemLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// CreateMenuItem bumps venues.menu_version in the same transaction as
// the insert (FR-C6). Availability windows (FR-C7) can't be set here —
// CreateMenuItemRequest carries no such field, only UpdateMenuItemRequest
// does (it sends the complete MenuItem) — so a new item always starts
// with none, meaning "always available" per GetMenu's window filtering.
func (l *CreateMenuItemLogic) CreateMenuItem(in *v1_catalogpb.CreateMenuItemRequest) (*v1_catalogpb.MenuItem, error) {
	name := in.GetName().GetTranslations()
	if len(name) == 0 {
		return nil, status.Error(codes.InvalidArgument, "name is required")
	}
	if in.GetCategoryId() == "" {
		return nil, status.Error(codes.InvalidArgument, "category_id is required")
	}
	if in.GetBasePrice().GetAmountMinor() < 0 {
		return nil, status.Error(codes.InvalidArgument, "base_price must not be negative")
	}

	venue, err := model.NewVenueModel(l.svcCtx.DB).FindByID(l.ctx, in.GetVenueId())
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			return nil, status.Error(codes.NotFound, "venue not found")
		}
		return nil, status.Errorf(codes.Internal, "look up venue: %v", err)
	}
	if c := in.GetBasePrice().GetCurrency(); c != "" && c != venue.Currency {
		return nil, status.Errorf(codes.InvalidArgument, "base_price currency %q does not match venue currency %q", c, venue.Currency)
	}

	var created *model.MenuItem
	err = l.svcCtx.DB.TransactCtx(l.ctx, func(ctx context.Context, session sqlx.Session) error {
		var err error
		created, err = model.NewMenuItemModel(session).Insert(ctx, in.GetVenueId(), in.GetCategoryId(),
			name, in.GetDescription().GetTranslations(), in.GetBasePrice().GetAmountMinor(),
			in.GetImageUrl(), in.GetAllergens(), in.GetSortOrder())
		if err != nil {
			return err
		}
		_, err = model.NewVenueModel(session).BumpMenuVersion(ctx, in.GetVenueId())
		return err
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "create menu item: %v", err)
	}

	item, err := hydrateItem(l.ctx, l.svcCtx.DB, created, venue.Currency)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "load modifiers: %v", err)
	}
	return item, nil
}
