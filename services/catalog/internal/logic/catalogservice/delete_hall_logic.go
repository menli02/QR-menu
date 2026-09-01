package catalogservicelogic

import (
	"context"

	"github.com/menli02/QR-menu/proto/catalog/v1"
	"github.com/menli02/QR-menu/services/catalog/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type DeleteHallLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewDeleteHallLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DeleteHallLogic {
	return &DeleteHallLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *DeleteHallLogic) DeleteHall(in *v1_catalogpb.DeleteHallRequest) (*v1_catalogpb.DeleteHallResponse, error) {
	// todo: add your logic here and delete this line

	return &v1_catalogpb.DeleteHallResponse{}, nil
}
