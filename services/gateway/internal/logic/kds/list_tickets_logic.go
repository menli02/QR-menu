// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package kds

import (
	"context"

	"github.com/menli02/QR-menu/services/gateway/internal/svc"
	"github.com/menli02/QR-menu/services/gateway/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListTicketsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewListTicketsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListTicketsLogic {
	return &ListTicketsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ListTicketsLogic) ListTickets(req *types.ListTicketsReq) (resp *types.ListTicketsResp, err error) {
	// todo: add your logic here and delete this line

	return
}
