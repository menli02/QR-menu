// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package admin

import (
	"context"

	v1_catalogpb "github.com/menli02/QR-menu/proto/catalog/v1"
	"github.com/menli02/QR-menu/services/gateway/internal/authz"
	"github.com/menli02/QR-menu/services/gateway/internal/convert"
	"github.com/menli02/QR-menu/services/gateway/internal/rpcerr"
	"github.com/menli02/QR-menu/services/gateway/internal/svc"
	"github.com/menli02/QR-menu/services/gateway/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListMenuItemsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewListMenuItemsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListMenuItemsLogic {
	return &ListMenuItemsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// ListMenuItems is the admin listing: inactive and unavailable items
// included, since managing them is the point.
func (l *ListMenuItemsLogic) ListMenuItems(req *types.ListMenuItemsReq) (resp *types.ListMenuItemsResp, err error) {
	claims, err := authz.Admin(l.ctx)
	if err != nil {
		return nil, err
	}

	settings, err := l.svcCtx.CatalogRpc.GetVenueSettings(l.ctx, &v1_catalogpb.GetVenueSettingsRequest{
		VenueId: claims.VenueID,
	})
	if err != nil {
		return nil, rpcerr.FromCatalog(err)
	}
	locale := settings.GetDefaultLocale()

	items, err := l.svcCtx.CatalogRpc.ListMenuItems(l.ctx, &v1_catalogpb.ListMenuItemsRequest{
		VenueId:    claims.VenueID,
		CategoryId: req.CategoryId,
		Cursor:     req.Cursor,
		PageSize:   req.PageSize,
	})
	if err != nil {
		return nil, rpcerr.FromCatalog(err)
	}

	return &types.ListMenuItemsResp{
		Items:      convert.MenuItems(items.GetItems(), locale, locale),
		NextCursor: items.GetNextCursor(),
	}, nil
}
