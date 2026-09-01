// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package guest

import (
	"context"

	"github.com/menli02/QR-menu/services/gateway/internal/svc"
	"github.com/menli02/QR-menu/services/gateway/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetGuestMenuLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetGuestMenuLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetGuestMenuLogic {
	return &GetGuestMenuLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetGuestMenuLogic) GetGuestMenu(req *types.GetGuestMenuReq) (resp *types.GetGuestMenuResp, err error) {
	// todo: add your logic here and delete this line

	return
}
