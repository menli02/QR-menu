package identityservicelogic

import (
	"context"

	"github.com/menli02/QR-menu/proto/identity/v1"
	"github.com/menli02/QR-menu/services/identity/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type UpdateStaffLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewUpdateStaffLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateStaffLogic {
	return &UpdateStaffLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *UpdateStaffLogic) UpdateStaff(in *v1_identitypb.UpdateStaffRequest) (*v1_identitypb.Staff, error) {
	// todo: add your logic here and delete this line

	return &v1_identitypb.Staff{}, nil
}
