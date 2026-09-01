package catalogservicelogic

import (
	"context"

	"github.com/menli02/QR-menu/proto/catalog/v1"
	"github.com/menli02/QR-menu/services/catalog/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type UpdateHallLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewUpdateHallLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateHallLogic {
	return &UpdateHallLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *UpdateHallLogic) UpdateHall(in *v1_catalogpb.UpdateHallRequest) (*v1_catalogpb.Hall, error) {
	// todo: add your logic here and delete this line

	return &v1_catalogpb.Hall{}, nil
}
