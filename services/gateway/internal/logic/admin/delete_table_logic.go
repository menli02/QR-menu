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

type DeleteTableLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewDeleteTableLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DeleteTableLogic {
	return &DeleteTableLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *DeleteTableLogic) DeleteTable(req *types.DeleteTableReq) (resp *types.DeletedResp, err error) {
	claims, err := authz.Admin(l.ctx)
	if err != nil {
		return nil, err
	}
	if req.Id == "" {
		return nil, errs.New(errs.CodeValidationFailed, "table id is required")
	}

	deleted, err := l.svcCtx.CatalogRpc.DeleteTable(l.ctx, &v1_catalogpb.DeleteTableRequest{
		VenueId: claims.VenueID,
		TableId: req.Id,
	})
	if err != nil {
		return nil, rpcerr.FromCatalog(err)
	}

	return &types.DeletedResp{Deleted: deleted.GetDeleted()}, nil
}
