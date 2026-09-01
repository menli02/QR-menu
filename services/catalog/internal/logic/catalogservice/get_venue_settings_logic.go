package catalogservicelogic

import (
	"context"

	"github.com/menli02/QR-menu/proto/catalog/v1"
	"github.com/menli02/QR-menu/services/catalog/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetVenueSettingsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetVenueSettingsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetVenueSettingsLogic {
	return &GetVenueSettingsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *GetVenueSettingsLogic) GetVenueSettings(in *v1_catalogpb.GetVenueSettingsRequest) (*v1_catalogpb.VenueSettings, error) {
	// todo: add your logic here and delete this line

	return &v1_catalogpb.VenueSettings{}, nil
}
