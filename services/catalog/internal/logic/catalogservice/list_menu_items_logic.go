package catalogservicelogic

import (
	"context"

	"github.com/menli02/QR-menu/proto/catalog/v1"
	"github.com/menli02/QR-menu/services/catalog/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListMenuItemsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListMenuItemsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListMenuItemsLogic {
	return &ListMenuItemsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ListMenuItemsLogic) ListMenuItems(in *v1_catalogpb.ListMenuItemsRequest) (*v1_catalogpb.ListMenuItemsResponse, error) {
	// todo: add your logic here and delete this line

	return &v1_catalogpb.ListMenuItemsResponse{}, nil
}
