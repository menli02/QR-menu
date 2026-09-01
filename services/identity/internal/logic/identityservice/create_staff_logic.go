package identityservicelogic

import (
	"context"

	"github.com/menli02/QR-menu/proto/identity/v1"
	"github.com/menli02/QR-menu/services/identity/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type CreateStaffLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCreateStaffLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateStaffLogic {
	return &CreateStaffLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *CreateStaffLogic) CreateStaff(in *v1_identitypb.CreateStaffRequest) (*v1_identitypb.Staff, error) {
	// todo: add your logic here and delete this line

	return &v1_identitypb.Staff{}, nil
}
