package catalogservicelogic

import (
	"context"

	"github.com/menli02/QR-menu/proto/catalog/v1"
	"github.com/menli02/QR-menu/services/catalog/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type CreateMenuItemLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCreateMenuItemLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateMenuItemLogic {
	return &CreateMenuItemLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *CreateMenuItemLogic) CreateMenuItem(in *v1_catalogpb.CreateMenuItemRequest) (*v1_catalogpb.MenuItem, error) {
	// todo: add your logic here and delete this line

	return &v1_catalogpb.MenuItem{}, nil
}
