package catalogservicelogic

import (
	"context"

	"github.com/menli02/QR-menu/proto/catalog/v1"
	"github.com/menli02/QR-menu/services/catalog/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type DeleteTableLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewDeleteTableLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DeleteTableLogic {
	return &DeleteTableLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *DeleteTableLogic) DeleteTable(in *v1_catalogpb.DeleteTableRequest) (*v1_catalogpb.DeleteTableResponse, error) {
	// todo: add your logic here and delete this line

	return &v1_catalogpb.DeleteTableResponse{}, nil
}
