// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package kds

import (
	"context"

	"github.com/menli02/QR-menu/services/gateway/internal/svc"
	"github.com/menli02/QR-menu/services/gateway/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type SetItemAvailabilityLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewSetItemAvailabilityLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SetItemAvailabilityLogic {
	return &SetItemAvailabilityLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *SetItemAvailabilityLogic) SetItemAvailability(req *types.SetItemAvailabilityReq) (resp *types.MenuItem, err error) {
	// todo: add your logic here and delete this line

	return
}
