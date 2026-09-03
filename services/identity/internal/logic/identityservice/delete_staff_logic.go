package identityservicelogic

import (
	"context"

	"github.com/menli02/QR-menu/proto/identity/v1"
	"github.com/menli02/QR-menu/services/identity/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
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

// DeleteStaff soft-deletes (is_active = false) rather than issuing a hard
// DELETE — see StaffModel.Deactivate's comment for why: other services
// hold staff_id as an opaque UUID with no FK, and a hard delete would
// orphan those references.
func (l *DeleteStaffLogic) DeleteStaff(in *v1_identitypb.DeleteStaffRequest) (*v1_identitypb.DeleteStaffResponse, error) {
	deleted, err := l.svcCtx.StaffModel.Deactivate(l.ctx, in.GetStaffId(), in.GetVenueId())
	if err != nil {
		return nil, status.Errorf(codes.Internal, "deactivate staff: %v", err)
	}
	return &v1_identitypb.DeleteStaffResponse{Deleted: deleted}, nil
}
