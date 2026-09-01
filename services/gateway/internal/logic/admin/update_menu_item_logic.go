// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package admin

import (
	"context"

	"github.com/menli02/QR-menu/services/gateway/internal/svc"
	"github.com/menli02/QR-menu/services/gateway/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type UpdateMenuItemLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewUpdateMenuItemLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateMenuItemLogic {
	return &UpdateMenuItemLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *UpdateMenuItemLogic) UpdateMenuItem(req *types.UpdateMenuItemReq) (resp *types.MenuItem, err error) {
	// todo: add your logic here and delete this line

	return
}
