// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package admin

import (
	"context"

	v1_catalogpb "github.com/menli02/QR-menu/proto/catalog/v1"
	"github.com/menli02/QR-menu/services/gateway/internal/authz"
	"github.com/menli02/QR-menu/services/gateway/internal/errs"
	"github.com/menli02/QR-menu/services/gateway/internal/rpcerr"
	"github.com/menli02/QR-menu/services/gateway/internal/svc"
	"github.com/menli02/QR-menu/services/gateway/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type DeleteMenuItemLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewDeleteMenuItemLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DeleteMenuItemLogic {
	return &DeleteMenuItemLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// DeleteMenuItem soft-deletes (is_active = false). Historical orders keep
// their own name and price snapshots, so a removed item never rewrites a
// bill someone already paid.
func (l *DeleteMenuItemLogic) DeleteMenuItem(req *types.DeleteMenuItemReq) (resp *types.DeletedResp, err error) {
	claims, err := authz.Admin(l.ctx)
	if err != nil {
		return nil, err
	}
	if req.Id == "" {
		return nil, errs.New(errs.CodeValidationFailed, "item id is required")
	}

	deleted, err := l.svcCtx.CatalogRpc.DeleteMenuItem(l.ctx, &v1_catalogpb.DeleteMenuItemRequest{
		VenueId: claims.VenueID,
		ItemId:  req.Id,
	})
	if err != nil {
		return nil, rpcerr.FromCatalog(err)
	}

	return &types.DeletedResp{Deleted: deleted.GetDeleted()}, nil
}
