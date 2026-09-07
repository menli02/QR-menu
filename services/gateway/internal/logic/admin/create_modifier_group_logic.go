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

type CreateModifierGroupLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewCreateModifierGroupLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateModifierGroupLogic {
	return &CreateModifierGroupLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *CreateModifierGroupLogic) CreateModifierGroup(req *types.CreateModifierGroupReq) (resp *types.ModifierGroup, err error) {
	claims, err := authz.Admin(l.ctx)
	if err != nil {
		return nil, err
	}
	if err := validateModifierGroup(req.ItemId, req.Name, req.MinSelect, req.MaxSelect, req.Required, len(req.Options)); err != nil {
		return nil, err
	}

	group, err := l.svcCtx.CatalogRpc.CreateModifierGroup(l.ctx, &v1_catalogpb.CreateModifierGroupRequest{
		VenueId:   claims.VenueID,
		ItemId:    req.ItemId,
		Name:      convert.LocalizedMap(req.Locale, req.Name),
		MinSelect: req.MinSelect,
		MaxSelect: req.MaxSelect,
		Required:  req.Required,
		Options:   modifierOptions(req.Locale, req.Options),
	})
	if err != nil {
		return nil, rpcerr.FromCatalog(err)
	}

	out := convert.ModifierGroup(group, req.Locale, req.Locale)
	return &out, nil
}
