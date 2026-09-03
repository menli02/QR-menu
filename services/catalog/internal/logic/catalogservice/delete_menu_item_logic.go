package catalogservicelogic

import (
	"context"

	"github.com/menli02/QR-menu/proto/catalog/v1"
	"github.com/menli02/QR-menu/services/catalog/internal/model"
	"github.com/menli02/QR-menu/services/catalog/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type DeleteMenuItemLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewDeleteMenuItemLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DeleteMenuItemLogic {
	return &DeleteMenuItemLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// DeleteMenuItem soft-deletes (is_active = false) — the proto field
// comment on MenuItem.is_active literally says "soft-deleted/published
// flag". Bumps menu_version: a hidden item is guest-visible content that
// just changed.
func (l *DeleteMenuItemLogic) DeleteMenuItem(in *v1_catalogpb.DeleteMenuItemRequest) (*v1_catalogpb.DeleteMenuItemResponse, error) {
	var deleted bool
	err := l.svcCtx.DB.TransactCtx(l.ctx, func(ctx context.Context, session sqlx.Session) error {
		var err error
		deleted, err = model.NewMenuItemModel(session).Deactivate(ctx, in.GetItemId(), in.GetVenueId())
		if err != nil {
			return err
		}
		_, err = model.NewVenueModel(session).BumpMenuVersion(ctx, in.GetVenueId())
		return err
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "deactivate menu item: %v", err)
	}
	return &v1_catalogpb.DeleteMenuItemResponse{Deleted: deleted}, nil
}
