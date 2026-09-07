package orderservicelogic

import (
	"context"

	"github.com/menli02/QR-menu/proto/order/v1"
	"github.com/menli02/QR-menu/services/order/internal/apierr"
	"github.com/menli02/QR-menu/services/order/internal/model"
	"github.com/menli02/QR-menu/services/order/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListOrdersLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListOrdersLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListOrdersLogic {
	return &ListOrdersLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// ListOrders returns every order on one table session, oldest first —
// the guest's "my orders" view (FR-O10) and the KDS/floor per-table
// drill-down.
//
// Cancelled orders are included: a guest who cancelled needs to see that
// it happened, and staff need it to answer "where did that ticket go".
// Unpaginated, because a table session is bounded by how much one party
// can order in a sitting.
func (l *ListOrdersLogic) ListOrders(in *v1_orderpb.ListOrdersRequest) (*v1_orderpb.ListOrdersResponse, error) {
	if in.GetVenueId() == "" || in.GetTableSessionId() == "" {
		return nil, apierr.Validation("venue_id and table_session_id are required")
	}

	orders, err := model.NewOrderModel(l.svcCtx.DB).ListByTableSession(l.ctx, in.GetVenueId(), in.GetTableSessionId())
	if err != nil {
		l.Errorf("list orders for session %s: %v", in.GetTableSessionId(), err)
		return nil, apierr.Internal("list orders")
	}

	out := &v1_orderpb.ListOrdersResponse{Orders: make([]*v1_orderpb.Order, 0, len(orders))}
	for i := range orders {
		out.Orders = append(out.Orders, orderToProto(&orders[i]))
	}
	return out, nil
}
