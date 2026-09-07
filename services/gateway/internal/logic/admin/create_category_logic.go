// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package admin

import (
	"context"

	v1_catalogpb "github.com/menli02/QR-menu/proto/catalog/v1"
	"github.com/menli02/QR-menu/services/gateway/internal/authz"
	"github.com/menli02/QR-menu/services/gateway/internal/convert"
	"github.com/menli02/QR-menu/services/gateway/internal/errs"
	"github.com/menli02/QR-menu/services/gateway/internal/rpcerr"
	"github.com/menli02/QR-menu/services/gateway/internal/svc"
	"github.com/menli02/QR-menu/services/gateway/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type CreateCategoryLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewCreateCategoryLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateCategoryLogic {
	return &CreateCategoryLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *CreateCategoryLogic) CreateCategory(req *types.CreateCategoryReq) (resp *types.Category, err error) {
	claims, err := authz.Admin(l.ctx)
	if err != nil {
		return nil, err
	}
	if req.Name == "" {
		return nil, errs.New(errs.CodeValidationFailed, "name is required")
	}

	category, err := l.svcCtx.CatalogRpc.CreateCategory(l.ctx, &v1_catalogpb.CreateCategoryRequest{
		VenueId:   claims.VenueID,
		Name:      convert.LocalizedMap(req.Locale, req.Name),
		SortOrder: req.SortOrder,
		IsVisible: req.IsVisible,
		ImageUrl:  req.ImageUrl,
	})
	if err != nil {
		return nil, rpcerr.FromCatalog(err)
	}

	out := convert.Category(category, req.Locale, req.Locale)
	return &out, nil
}
