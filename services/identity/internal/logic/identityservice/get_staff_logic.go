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

// GetStaff fetches one staff record by id.
//
// SECURITY (flagged, not fixed here): GetStaffRequest carries no venue_id
// (see identity.proto), so this cannot verify the caller's own venue
// matches the returned row's venue_id — unlike Update/SetPassword/
// Deactivate below, which are all scoped to (id, venue_id). If any caller
// ever passes this a foreign staff_id, it leaks that staff member's name/
// email/role across the tenant boundary. IDs are random UUIDv4
// (unguessable on their own), which limits exploitability, but this
// should be closed with an additive `venue_id` field on GetStaffRequest —
// that's a proto/contract change (docs/TZ.md §8 treats these as
// long-lived interfaces) and deserves its own visible commit rather than
// being folded in here silently.
func (l *GetStaffLogic) GetStaff(in *v1_identitypb.GetStaffRequest) (*v1_identitypb.Staff, error) {
	staff, err := l.svcCtx.StaffModel.FindByID(l.ctx, in.GetStaffId())
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			return nil, status.Error(codes.NotFound, "staff not found")
		}
		return nil, status.Errorf(codes.Internal, "look up staff: %v", err)
	}
	return staffToProto(staff), nil
}
