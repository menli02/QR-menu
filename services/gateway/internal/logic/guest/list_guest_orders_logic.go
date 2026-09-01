// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package guest

import (
	"context"

	"github.com/menli02/QR-menu/services/gateway/internal/svc"
	"github.com/menli02/QR-menu/services/gateway/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListGuestOrdersLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewListGuestOrdersLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListGuestOrdersLogic {
	return &ListGuestOrdersLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ListGuestOrdersLogic) ListGuestOrders() (resp *types.ListGuestOrdersResp, err error) {
	// todo: add your logic here and delete this line

	return
}
