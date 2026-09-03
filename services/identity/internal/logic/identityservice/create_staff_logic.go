package identityservicelogic

import (
	"context"

	"github.com/menli02/QR-menu/proto/identity/v1"
	"github.com/menli02/QR-menu/services/identity/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// minPasswordLen is a floor, not the full policy — FR-A1's password
// policy (docs/TZ.md §5.6, §11.3) is not otherwise specified yet; this
// guards against the obviously-too-weak case.
const minPasswordLen = 8

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
	role, err := roleToDB(in.GetRole())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "role must be specified")
	}
	if in.GetName() == "" || in.GetEmail() == "" {
		return nil, status.Error(codes.InvalidArgument, "name and email are required")
	}
	if len(in.GetPassword()) < minPasswordLen {
		return nil, status.Errorf(codes.InvalidArgument, "password must be at least %d characters", minPasswordLen)
	}

	hash, err := hashPassword(in.GetPassword())
	if err != nil {
		return nil, status.Errorf(codes.Internal, "hash password: %v", err)
	}

	staff, err := l.svcCtx.StaffModel.Insert(l.ctx, in.GetVenueId(), in.GetName(), in.GetEmail(), hash, role)
	if err != nil {
		if isUniqueViolation(err) {
			return nil, status.Error(codes.AlreadyExists, "a staff member with this email already exists in this venue")
		}
		return nil, status.Errorf(codes.Internal, "create staff: %v", err)
	}
	return staffToProto(staff), nil
}
