package orderservicelogic

import (
	"context"

	"github.com/menli02/QR-menu/proto/order/v1"
	"github.com/menli02/QR-menu/services/order/internal/apierr"
	"github.com/menli02/QR-menu/services/order/internal/model"
	"github.com/menli02/QR-menu/services/order/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListActiveTableSessionsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListActiveTableSessionsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListActiveTableSessionsLogic {
	return &ListActiveTableSessionsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// ListActiveTableSessions backs the floor view's table map (§8.1
// GET /floor/tables): every occupied table with its running total.
//
// The totals come from table_sessions.total_minor, the denormalized
// mirror, precisely because this view wants tens of rows at a glance and
// must not do a per-table bill computation to render. GetBill remains the
// authority for the number a guest actually pays.
func (l *ListActiveTableSessionsLogic) ListActiveTableSessions(in *v1_orderpb.ListActiveTableSessionsRequest) (*v1_orderpb.ListActiveTableSessionsResponse, error) {
	if in.GetVenueId() == "" {
		return nil, apierr.Validation("venue_id is required")
	}

	sessions, err := model.NewTableSessionModel(l.svcCtx.DB).ListActive(l.ctx, in.GetVenueId())
	if err != nil {
		l.Errorf("list active sessions for venue %s: %v", in.GetVenueId(), err)
		return nil, apierr.Internal("list active table sessions")
	}

	out := &v1_orderpb.ListActiveTableSessionsResponse{
		Sessions: make([]*v1_orderpb.TableSession, 0, len(sessions)),
	}
	for i := range sessions {
		out.Sessions = append(out.Sessions, tableSessionToProto(&sessions[i]))
	}
	return out, nil
}
