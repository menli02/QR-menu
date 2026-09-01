package catalogservicelogic

import (
	"context"

	"github.com/menli02/QR-menu/proto/catalog/v1"
	"github.com/menli02/QR-menu/services/catalog/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type UpdateMenuItemLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewUpdateMenuItemLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateMenuItemLogic {
	return &UpdateMenuItemLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *UpdateMenuItemLogic) UpdateMenuItem(in *v1_catalogpb.UpdateMenuItemRequest) (*v1_catalogpb.MenuItem, error) {
	// todo: add your logic here and delete this line

	return &v1_catalogpb.MenuItem{}, nil
}
