// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package admin

import (
	"context"

	"github.com/menli02/QR-menu/services/gateway/internal/svc"
	"github.com/menli02/QR-menu/services/gateway/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type CreateMenuItemLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewCreateMenuItemLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateMenuItemLogic {
	return &CreateMenuItemLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *CreateMenuItemLogic) CreateMenuItem(req *types.CreateMenuItemReq) (resp *types.MenuItem, err error) {
	// todo: add your logic here and delete this line

	return
}
