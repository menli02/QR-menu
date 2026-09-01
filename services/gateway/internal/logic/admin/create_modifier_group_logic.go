// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package admin

import (
	"context"

	"github.com/menli02/QR-menu/services/gateway/internal/svc"
	"github.com/menli02/QR-menu/services/gateway/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type CreateModifierGroupLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewCreateModifierGroupLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateModifierGroupLogic {
	return &CreateModifierGroupLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *CreateModifierGroupLogic) CreateModifierGroup(req *types.CreateModifierGroupReq) (resp *types.ModifierGroup, err error) {
	// todo: add your logic here and delete this line

	return
}
