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

type DeleteModifierGroupLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewDeleteModifierGroupLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DeleteModifierGroupLogic {
	return &DeleteModifierGroupLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// DeleteModifierGroup is a hard delete: its options cascade, and order
// history keeps its own snapshots of any modifier ever ordered, so nothing
// already sold is affected.
func (l *DeleteModifierGroupLogic) DeleteModifierGroup(req *types.DeleteModifierGroupReq) (resp *types.DeletedResp, err error) {
	claims, err := authz.Admin(l.ctx)
	if err != nil {
		return nil, err
	}
	if req.ItemId == "" || req.GroupId == "" {
		return nil, errs.New(errs.CodeValidationFailed, "itemId and groupId are required")
	}

	deleted, err := l.svcCtx.CatalogRpc.DeleteModifierGroup(l.ctx, &v1_catalogpb.DeleteModifierGroupRequest{
		VenueId: claims.VenueID,
		ItemId:  req.ItemId,
		GroupId: req.GroupId,
	})
	if err != nil {
		return nil, rpcerr.FromCatalog(err)
	}

	return &types.DeletedResp{Deleted: deleted.GetDeleted()}, nil
}
