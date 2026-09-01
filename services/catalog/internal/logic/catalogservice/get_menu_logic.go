package catalogservicelogic

import (
	"context"

	"github.com/menli02/QR-menu/proto/catalog/v1"
	"github.com/menli02/QR-menu/services/catalog/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetMenuLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetMenuLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetMenuLogic {
	return &GetMenuLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// GetMenu returns the full active, visible, in-locale menu for one venue
func (l *GetMenuLogic) GetMenu(in *v1_catalogpb.GetMenuRequest) (*v1_catalogpb.GetMenuResponse, error) {
	// todo: add your logic here and delete this line

	return &v1_catalogpb.GetMenuResponse{}, nil
}
