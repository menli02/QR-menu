package identityservicelogic

import (
	"context"

	"github.com/menli02/QR-menu/proto/identity/v1"
	"github.com/menli02/QR-menu/services/identity/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type DeleteStaffLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewDeleteStaffLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DeleteStaffLogic {
	return &DeleteStaffLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *DeleteStaffLogic) DeleteStaff(in *v1_identitypb.DeleteStaffRequest) (*v1_identitypb.DeleteStaffResponse, error) {
	// todo: add your logic here and delete this line

	return &v1_identitypb.DeleteStaffResponse{}, nil
}
