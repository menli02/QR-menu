package identityservicelogic

import (
	"context"
	"errors"

	"github.com/menli02/QR-menu/proto/identity/v1"
	"github.com/menli02/QR-menu/services/identity/internal/model"
	"github.com/menli02/QR-menu/services/identity/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
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

// UpdateStaff changes profile fields (name, email, role, is_active) —
// never the password, which only SetStaffPassword touches. Scoped to
// (staff.id, staff.venue_id): a request whose Staff.venue_id doesn't
// match the row's real venue affects nothing rather than reassigning
// staff across venues.
func (l *UpdateStaffLogic) UpdateStaff(in *v1_identitypb.UpdateStaffRequest) (*v1_identitypb.Staff, error) {
	s := in.GetStaff()
	if s == nil {
		return nil, status.Error(codes.InvalidArgument, "staff is required")
	}
	role, err := roleToDB(s.GetRole())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "role must be specified")
	}
	if s.GetName() == "" || s.GetEmail() == "" {
		return nil, status.Error(codes.InvalidArgument, "name and email are required")
	}

	updated, err := l.svcCtx.StaffModel.Update(l.ctx, s.GetId(), s.GetVenueId(), s.GetName(), s.GetEmail(), role, s.GetIsActive())
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			return nil, status.Error(codes.NotFound, "staff not found")
		}
		if isUniqueViolation(err) {
			return nil, status.Error(codes.AlreadyExists, "a staff member with this email already exists in this venue")
		}
		return nil, status.Errorf(codes.Internal, "update staff: %v", err)
	}
	return staffToProto(updated), nil
}
