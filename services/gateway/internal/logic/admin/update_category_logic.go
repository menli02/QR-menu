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

type UpdateCategoryLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewUpdateCategoryLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateCategoryLogic {
	return &UpdateCategoryLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// UpdateCategory edits one locale's name plus the non-localized fields.
//
// Sending a single-locale map is safe: catalog merges name into the stored
// translations with JSONB `||` rather than replacing them, so editing the
// English name leaves the Russian one intact. Doing that merge server-side
// also avoids the lost-update race a read-modify-write here would have
// between two admins editing two different locales at once.
func (l *UpdateCategoryLogic) UpdateCategory(req *types.UpdateCategoryReq) (resp *types.Category, err error) {
	claims, err := authz.Admin(l.ctx)
	if err != nil {
		return nil, err
	}
	if req.Id == "" || req.Name == "" {
		return nil, errs.New(errs.CodeValidationFailed, "category id and name are required")
	}

	category, err := l.svcCtx.CatalogRpc.UpdateCategory(l.ctx, &v1_catalogpb.UpdateCategoryRequest{
		Category: &v1_catalogpb.Category{
			Id:        req.Id,
			VenueId:   claims.VenueID,
			Name:      convert.LocalizedMap(req.Locale, req.Name),
			SortOrder: req.SortOrder,
			IsVisible: req.IsVisible,
			ImageUrl:  req.ImageUrl,
		},
	})
	if err != nil {
		return nil, rpcerr.FromCatalog(err)
	}

	out := convert.Category(category, req.Locale, req.Locale)
	return &out, nil
}
