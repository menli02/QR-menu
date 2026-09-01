// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package admin

import (
	"context"

	"github.com/menli02/QR-menu/services/gateway/internal/svc"
	"github.com/menli02/QR-menu/services/gateway/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type UpdateVenueSettingsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewUpdateVenueSettingsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateVenueSettingsLogic {
	return &UpdateVenueSettingsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *UpdateVenueSettingsLogic) UpdateVenueSettings(req *types.UpdateVenueSettingsReq) (resp *types.VenueSettings, err error) {
	// todo: add your logic here and delete this line

	return
}
