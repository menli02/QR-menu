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

type UpdateMenuItemLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewUpdateMenuItemLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateMenuItemLogic {
	return &UpdateMenuItemLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// UpdateMenuItem edits one locale's text plus the non-localized fields.
// Catalog merges the translation maps — see UpdateCategory for why that
// happens there rather than here.
//
// isAvailable is absent on purpose: the stop-list toggle has its own
// endpoint (POST /kds/items/{id}/availability, FR-C4/FR-K6) that any staff
// role may use, and routing it through this manager-only form would take
// it away from the cook who needs it.
func (l *UpdateMenuItemLogic) UpdateMenuItem(req *types.UpdateMenuItemReq) (resp *types.MenuItem, err error) {
	claims, err := authz.Admin(l.ctx)
	if err != nil {
		return nil, err
	}
	if req.Id == "" || req.CategoryId == "" || req.Name == "" {
		return nil, errs.New(errs.CodeValidationFailed, "item id, categoryId and name are required")
	}
	if req.BasePrice.AmountMinor < 0 {
		return nil, errs.New(errs.CodeValidationFailed, "basePrice cannot be negative")
	}

	item, err := l.svcCtx.CatalogRpc.UpdateMenuItem(l.ctx, &v1_catalogpb.UpdateMenuItemRequest{
		Item: &v1_catalogpb.MenuItem{
			Id:          req.Id,
			VenueId:     claims.VenueID,
			CategoryId:  req.CategoryId,
			Name:        convert.LocalizedMap(req.Locale, req.Name),
			Description: convert.LocalizedMap(req.Locale, req.Description),
			BasePrice:   &v1_catalogpb.Money{AmountMinor: req.BasePrice.AmountMinor},
			ImageUrl:    req.ImageUrl,
			Allergens:   req.Allergens,
			IsActive:    req.IsActive,
			SortOrder:   req.SortOrder,
		},
	})
	if err != nil {
		return nil, rpcerr.FromCatalog(err)
	}

	out := convert.MenuItem(item, req.Locale, req.Locale)
	return &out, nil
}
