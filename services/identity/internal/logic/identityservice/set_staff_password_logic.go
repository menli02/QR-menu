package identityservicelogic

import (
	"context"

	"github.com/menli02/QR-menu/proto/identity/v1"
	"github.com/menli02/QR-menu/services/identity/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type SetStaffPasswordLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewSetStaffPasswordLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SetStaffPasswordLogic {
	return &SetStaffPasswordLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *SetStaffPasswordLogic) SetStaffPassword(in *v1_identitypb.SetStaffPasswordRequest) (*v1_identitypb.SetStaffPasswordResponse, error) {
	// todo: add your logic here and delete this line

	return &v1_identitypb.SetStaffPasswordResponse{}, nil
}
