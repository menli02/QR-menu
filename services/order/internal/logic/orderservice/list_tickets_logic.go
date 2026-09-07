package orderservicelogic

import (
	"context"

	"github.com/menli02/QR-menu/proto/order/v1"
	"github.com/menli02/QR-menu/services/order/internal/apierr"
	"github.com/menli02/QR-menu/services/order/internal/model"
	"github.com/menli02/QR-menu/services/order/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListTicketsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListTicketsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListTicketsLogic {
	return &ListTicketsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// ListTickets is the KDS ticket list (FR-K1): oldest first, so the ticket
// that has been waiting longest is at the top of the screen.
//
// An empty status_filter means "active" (placed through ready), per the
// field's proto comment — not "everything". A kitchen screen that
// defaulted to showing every served ticket from the whole day would be
// useless, and it is also the query that would hurt most.
//
// Pagination is keyset on (placed_at, id), not OFFSET: the list is
// append-mostly and constantly re-fetched, and OFFSET would skip or
// duplicate tickets as new ones arrive underneath a paging client.
//
// FR-K4's colour escalation is deliberately not computed here. The
// thresholds are venue settings the KDS client already holds, placed_at
// is on every ticket, and a server-computed colour would be stale the
// moment it was rendered.
func (l *ListTicketsLogic) ListTickets(in *v1_orderpb.ListTicketsRequest) (*v1_orderpb.ListTicketsResponse, error) {
	if in.GetVenueId() == "" {
		return nil, apierr.Validation("venue_id is required")
	}

	statuses := make([]string, 0, len(in.GetStatusFilter()))
	for _, s := range in.GetStatusFilter() {
		mapped, ok := orderStatusFromProto[s]
		if !ok {
			return nil, apierr.Validation("status_filter contains an unknown order status")
		}
		statuses = append(statuses, mapped)
	}

	after, err := model.DecodeCursor(in.GetCursor())
	if err != nil {
		return nil, apierr.Validation("cursor is not a valid pagination cursor")
	}
	limit := clampPageSize(in.GetPageSize())

	// Fetch one extra row to learn whether another page exists without a
	// second count query.
	orders, err := model.NewOrderModel(l.svcCtx.DB).ListTickets(l.ctx, in.GetVenueId(), statuses, after, limit+1)
	if err != nil {
		l.Errorf("list tickets for venue %s: %v", in.GetVenueId(), err)
		return nil, apierr.Internal("list tickets")
	}

	out := &v1_orderpb.ListTicketsResponse{}
	if len(orders) > limit {
		orders = orders[:limit]
		last := orders[len(orders)-1]
		out.NextCursor = model.Cursor{Timestamp: last.PlacedAt, ID: last.ID}.Encode()
	}

	out.Orders = make([]*v1_orderpb.Order, 0, len(orders))
	for i := range orders {
		out.Orders = append(out.Orders, orderToProto(&orders[i]))
	}
	return out, nil
}
