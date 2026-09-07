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

type DeleteCategoryLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewDeleteCategoryLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DeleteCategoryLogic {
	return &DeleteCategoryLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// DeleteCategory removes a category. Catalog refuses while menu items
// still reference it (ON DELETE RESTRICT), which surfaces as
// VALIDATION_FAILED rather than orphaning a menu.
func (l *DeleteCategoryLogic) DeleteCategory(req *types.DeleteCategoryReq) (resp *types.DeletedResp, err error) {
	claims, err := authz.Admin(l.ctx)
	if err != nil {
		return nil, err
	}
	if req.Id == "" {
		return nil, errs.New(errs.CodeValidationFailed, "category id is required")
	}

	deleted, err := l.svcCtx.CatalogRpc.DeleteCategory(l.ctx, &v1_catalogpb.DeleteCategoryRequest{
		VenueId:    claims.VenueID,
		CategoryId: req.Id,
	})
	if err != nil {
		return nil, rpcerr.FromCatalog(err)
	}

	return &types.DeletedResp{Deleted: deleted.GetDeleted()}, nil
}
