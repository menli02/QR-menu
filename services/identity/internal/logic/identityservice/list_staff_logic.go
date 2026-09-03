package identityservicelogic

import (
	"context"

	"github.com/menli02/QR-menu/proto/identity/v1"
	"github.com/menli02/QR-menu/services/identity/internal/model"
	"github.com/menli02/QR-menu/services/identity/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	defaultStaffPageSize = 50
	maxStaffPageSize     = 200
)

type ListStaffLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListStaffLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListStaffLogic {
	return &ListStaffLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// ListStaff is keyset-paginated (model.Cursor), not offset-based: correct
// under concurrent staff creation and no slower on a deep page.
func (l *ListStaffLogic) ListStaff(in *v1_identitypb.ListStaffRequest) (*v1_identitypb.ListStaffResponse, error) {
	cursor, err := model.DecodeCursor(in.GetCursor())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid cursor: %v", err)
	}

	limit := int(in.GetPageSize())
	if limit <= 0 || limit > maxStaffPageSize {
		limit = defaultStaffPageSize
	}

	rows, err := l.svcCtx.StaffModel.List(l.ctx, in.GetVenueId(), cursor, limit)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list staff: %v", err)
	}

	resp := &v1_identitypb.ListStaffResponse{Staff: make([]*v1_identitypb.Staff, 0, len(rows))}
	for i := range rows {
		resp.Staff = append(resp.Staff, staffToProto(&rows[i]))
	}
	// A full page optimistically implies there may be more; the client's
	// next call with this cursor simply comes back empty if not. Precise
	// "is there really a next page" would mean fetching limit+1 rows —
	// not worth the extra row for a page listing staff (dozens, not
	// thousands, per venue).
	if len(rows) == limit {
		last := rows[len(rows)-1]
		resp.NextCursor = model.Cursor{CreatedAt: last.CreatedAt, ID: last.ID}.Encode()
	}
	return resp, nil
}
