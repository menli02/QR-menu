// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package admin

import (
	"context"

	"github.com/menli02/QR-menu/services/gateway/internal/svc"
	"github.com/menli02/QR-menu/services/gateway/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type DeleteModifierGroupLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewDeleteModifierGroupLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DeleteModifierGroupLogic {
	return &DeleteModifierGroupLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *DeleteModifierGroupLogic) DeleteModifierGroup(req *types.DeleteModifierGroupReq) (resp *types.DeletedResp, err error) {
	// todo: add your logic here and delete this line

	return
}
