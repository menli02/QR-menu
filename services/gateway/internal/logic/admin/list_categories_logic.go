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

type ListCategoriesLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewListCategoriesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListCategoriesLogic {
	return &ListCategoriesLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// ListCategories is the admin listing: every category, hidden ones
// included, unlike the guest menu.
//
// Localized names are resolved to the venue's default locale. The public
// Category type carries a single `name` string (§8.1's admin bodies edit
// one locale at a time), so an admin editing a non-default locale reads it
// back through the same locale-scoped write it sent. A multi-locale editor
// would need the full map, which is an additive contract change.
func (l *ListCategoriesLogic) ListCategories() (resp *types.ListCategoriesResp, err error) {
	claims, err := authz.Admin(l.ctx)
	if err != nil {
		return nil, err
	}

	defaultLocale, err := l.venueDefaultLocale(claims.VenueID)
	if err != nil {
		return nil, err
	}

	categories, err := l.svcCtx.CatalogRpc.ListCategories(l.ctx, &v1_catalogpb.ListCategoriesRequest{
		VenueId: claims.VenueID,
	})
	if err != nil {
		return nil, rpcerr.FromCatalog(err)
	}

	return &types.ListCategoriesResp{
		Categories: convert.Categories(categories.GetCategories(), defaultLocale, defaultLocale),
	}, nil
}

func (l *ListCategoriesLogic) venueDefaultLocale(venueID string) (string, error) {
	settings, err := l.svcCtx.CatalogRpc.GetVenueSettings(l.ctx, &v1_catalogpb.GetVenueSettingsRequest{VenueId: venueID})
	if err != nil {
		return "", rpcerr.FromCatalog(err)
	}
	return settings.GetDefaultLocale(), nil
}
