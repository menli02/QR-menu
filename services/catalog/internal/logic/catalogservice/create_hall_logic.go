package catalogservicelogic

import (
	"context"

	"github.com/menli02/QR-menu/proto/catalog/v1"
	"github.com/menli02/QR-menu/services/catalog/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type CreateHallLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCreateHallLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateHallLogic {
	return &CreateHallLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *CreateHallLogic) CreateHall(in *v1_catalogpb.CreateHallRequest) (*v1_catalogpb.Hall, error) {
	// todo: add your logic here and delete this line

	return &v1_catalogpb.Hall{}, nil
}
