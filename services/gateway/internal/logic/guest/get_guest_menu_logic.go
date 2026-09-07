// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package guest

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

type GetGuestMenuLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetGuestMenuLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetGuestMenuLogic {
	return &GetGuestMenuLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// GetGuestMenu returns the venue's menu in one response (FR-C5).
//
// The venue comes from the guest's token, never from the request: a guest
// authenticated for one venue must not be able to read another's menu by
// changing a query parameter, and taking it from the claims makes that
// impossible rather than merely checked.
//
// §8.1 specifies ETag + Cache-Control on this route. Neither is set here:
// goctl's logic signature has no access to the ResponseWriter, and
// catalog's menu_version — which is exactly the ETag value — is returned
// in the body instead. Wiring the real headers needs a small custom
// handler; flagged rather than half-done, since a wrong ETag is worse
// than none.
func (l *GetGuestMenuLogic) GetGuestMenu(req *types.GetGuestMenuReq) (resp *types.GetGuestMenuResp, err error) {
	claims, err := authz.Guest(l.ctx)
	if err != nil {
		return nil, err
	}

	menu, err := l.svcCtx.CatalogRpc.GetMenu(l.ctx, &v1_catalogpb.GetMenuRequest{
		VenueId: claims.VenueID,
		Locale:  req.Locale,
	})
	if err != nil {
		return nil, rpcerr.FromCatalog(err)
	}

	out := &types.GetGuestMenuResp{
		VenueId:     menu.GetVenueId(),
		MenuVersion: menu.GetMenuVersion(),
		Currency:    menu.GetCurrency(),
		Categories:  make([]types.MenuCategory, 0, len(menu.GetCategories())),
	}
	for _, c := range menu.GetCategories() {
		category := c.GetCategory()
		out.Categories = append(out.Categories, types.MenuCategory{
			Id:        category.GetId(),
			Name:      convert.Localized(category.GetName(), req.Locale, ""),
			SortOrder: category.GetSortOrder(),
			Items:     convert.MenuItems(c.GetItems(), req.Locale, ""),
		})
	}
	return out, nil
}
