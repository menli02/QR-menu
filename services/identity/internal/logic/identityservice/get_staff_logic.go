package identityservicelogic

import (
	"context"

	"github.com/menli02/QR-menu/proto/identity/v1"
	"github.com/menli02/QR-menu/services/identity/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetStaffLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetStaffLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetStaffLogic {
	return &GetStaffLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *GetStaffLogic) GetStaff(in *v1_identitypb.GetStaffRequest) (*v1_identitypb.Staff, error) {
	// todo: add your logic here and delete this line

	return &v1_identitypb.Staff{}, nil
}
