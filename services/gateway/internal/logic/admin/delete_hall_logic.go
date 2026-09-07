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

type DeleteHallLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewDeleteHallLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DeleteHallLogic {
	return &DeleteHallLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// DeleteHall removes a hall. Catalog refuses if tables still reference it
// (ON DELETE RESTRICT) — deleting a hall out from under a live table map
// is not something to do silently.
func (l *DeleteHallLogic) DeleteHall(req *types.DeleteHallReq) (resp *types.DeletedResp, err error) {
	claims, err := authz.Admin(l.ctx)
	if err != nil {
		return nil, err
	}
	if req.Id == "" {
		return nil, errs.New(errs.CodeValidationFailed, "hall id is required")
	}

	deleted, err := l.svcCtx.CatalogRpc.DeleteHall(l.ctx, &v1_catalogpb.DeleteHallRequest{
		VenueId: claims.VenueID,
		HallId:  req.Id,
	})
	if err != nil {
		return nil, rpcerr.FromCatalog(err)
	}

	return &types.DeletedResp{Deleted: deleted.GetDeleted()}, nil
}
