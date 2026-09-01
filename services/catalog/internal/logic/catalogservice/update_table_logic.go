package catalogservicelogic

import (
	"context"

	"github.com/menli02/QR-menu/proto/catalog/v1"
	"github.com/menli02/QR-menu/services/catalog/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type UpdateTableLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewUpdateTableLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateTableLogic {
	return &UpdateTableLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *UpdateTableLogic) UpdateTable(in *v1_catalogpb.UpdateTableRequest) (*v1_catalogpb.Table, error) {
	// todo: add your logic here and delete this line

	return &v1_catalogpb.Table{}, nil
}
