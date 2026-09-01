// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package admin

import (
	"context"

	"github.com/menli02/QR-menu/services/gateway/internal/svc"
	"github.com/menli02/QR-menu/services/gateway/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetVenueSettingsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetVenueSettingsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetVenueSettingsLogic {
	return &GetVenueSettingsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetVenueSettingsLogic) GetVenueSettings() (resp *types.VenueSettings, err error) {
	// todo: add your logic here and delete this line

	return
}
