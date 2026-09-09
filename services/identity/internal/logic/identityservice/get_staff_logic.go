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

// GetStaff fetches one staff record, scoped to a venue.
//
// The venue_id is required, not optional. It was added to the request
// after the fact (identity.proto field 2) precisely to close a
// cross-tenant read: before it, this was the one staff RPC not scoped to
// (id, venue_id), so any caller holding a staff id from another venue got
// back that person's name, email and role. Rejecting an empty venue_id
// rather than treating it as "any venue" is what makes the fix real — a
// caller that has not been updated fails loudly instead of silently
// keeping the old behaviour.
func (l *GetStaffLogic) GetStaff(in *v1_identitypb.GetStaffRequest) (*v1_identitypb.Staff, error) {
	if in.GetStaffId() == "" || in.GetVenueId() == "" {
		return nil, status.Error(codes.InvalidArgument, "staff_id and venue_id are required")
	}

	staff, err := l.svcCtx.StaffModel.FindByID(l.ctx, in.GetVenueId(), in.GetStaffId())
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			return nil, status.Error(codes.NotFound, "staff not found")
		}
		return nil, status.Errorf(codes.Internal, "look up staff: %v", err)
	}
	return staffToProto(staff), nil
}
