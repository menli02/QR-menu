// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package admin

import (
	"context"

	"github.com/menli02/QR-menu/services/gateway/internal/svc"
	"github.com/menli02/QR-menu/services/gateway/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type UpdateStaffLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewUpdateStaffLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateStaffLogic {
	return &UpdateStaffLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *UpdateStaffLogic) UpdateStaff(req *types.UpdateStaffReq) (resp *types.Staff, err error) {
	// todo: add your logic here and delete this line

	return
}
