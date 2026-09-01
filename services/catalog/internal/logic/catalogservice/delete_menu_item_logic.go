package catalogservicelogic

import (
	"context"

	"github.com/menli02/QR-menu/proto/catalog/v1"
	"github.com/menli02/QR-menu/services/catalog/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type DeleteMenuItemLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewDeleteMenuItemLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DeleteMenuItemLogic {
	return &DeleteMenuItemLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *DeleteMenuItemLogic) DeleteMenuItem(in *v1_catalogpb.DeleteMenuItemRequest) (*v1_catalogpb.DeleteMenuItemResponse, error) {
	// todo: add your logic here and delete this line

	return &v1_catalogpb.DeleteMenuItemResponse{}, nil
}
