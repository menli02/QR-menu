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

type CreateMenuItemLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewCreateMenuItemLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateMenuItemLogic {
	return &CreateMenuItemLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *CreateMenuItemLogic) CreateMenuItem(req *types.CreateMenuItemReq) (resp *types.MenuItem, err error) {
	claims, err := authz.Admin(l.ctx)
	if err != nil {
		return nil, err
	}
	if req.CategoryId == "" || req.Name == "" {
		return nil, errs.New(errs.CodeValidationFailed, "categoryId and name are required")
	}
	if req.BasePrice.AmountMinor < 0 {
		return nil, errs.New(errs.CodeValidationFailed, "basePrice cannot be negative")
	}

	// The item's currency is the venue's (A1) — catalog stores no
	// per-item currency, so whatever the client sent in basePrice.currency
	// is ignored rather than silently persisted as a second source of
	// truth.
	item, err := l.svcCtx.CatalogRpc.CreateMenuItem(l.ctx, &v1_catalogpb.CreateMenuItemRequest{
		VenueId:     claims.VenueID,
		CategoryId:  req.CategoryId,
		Name:        convert.LocalizedMap(req.Locale, req.Name),
		Description: convert.LocalizedMap(req.Locale, req.Description),
		BasePrice:   &v1_catalogpb.Money{AmountMinor: req.BasePrice.AmountMinor},
		ImageUrl:    req.ImageUrl,
		Allergens:   req.Allergens,
		SortOrder:   req.SortOrder,
	})
	if err != nil {
		return nil, rpcerr.FromCatalog(err)
	}

	out := convert.MenuItem(item, req.Locale, req.Locale)
	return &out, nil
}
