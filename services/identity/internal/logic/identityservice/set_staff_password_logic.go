package identityservicelogic

import (
	"context"

	"github.com/menli02/QR-menu/proto/identity/v1"
	"github.com/menli02/QR-menu/services/identity/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
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
	if len(in.GetNewPassword()) < minPasswordLen {
		return nil, status.Errorf(codes.InvalidArgument, "password must be at least %d characters", minPasswordLen)
	}

	hash, err := hashPassword(in.GetNewPassword())
	if err != nil {
		return nil, status.Errorf(codes.Internal, "hash password: %v", err)
	}

	updated, err := l.svcCtx.StaffModel.SetPassword(l.ctx, in.GetStaffId(), in.GetVenueId(), hash)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "set password: %v", err)
	}
	return &v1_identitypb.SetStaffPasswordResponse{Updated: updated}, nil
}
