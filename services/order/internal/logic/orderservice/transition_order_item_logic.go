package orderservicelogic

import (
	"context"
	"fmt"

	"github.com/menli02/QR-menu/proto/order/v1"
	"github.com/menli02/QR-menu/services/order/internal/apierr"
	"github.com/menli02/QR-menu/services/order/internal/model"
	"github.com/menli02/QR-menu/services/order/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

type TransitionOrderItemLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewTransitionOrderItemLogic(ctx context.Context, svcCtx *svc.ServiceContext) *TransitionOrderItemLogic {
	return &TransitionOrderItemLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// TransitionOrderItem moves one line of a ticket (FR-K3's per-item
// actions) and returns the whole order, because a line change can move
// the order too.
//
// Two derived effects, both applied in the same transaction as the line
// change so a consumer never sees one without the other:
//
//   - FR-K5: once every non-cancelled line is ready, the order becomes
//     ready by itself. Cancel every line and the order cancels.
//   - A cancelled line stops being billed, so the order total and the
//     table session's running total are both corrected.
func (l *TransitionOrderItemLogic) TransitionOrderItem(in *v1_orderpb.TransitionOrderItemRequest) (*v1_orderpb.Order, error) {
	if in.GetVenueId() == "" || in.GetOrderId() == "" || in.GetOrderItemId() == "" {
		return nil, apierr.Validation("venue_id, order_id and order_item_id are required")
	}
	to, ok := itemStatusFromProto[in.GetTo()]
	if !ok {
		return nil, apierr.Validation("to must be a known order item status")
	}

	fingerprint := model.Fingerprint(in.GetVenueId(), in.GetOrderId(), in.GetOrderItemId(), to, in.GetActorStaffId())

	if replay, found, err := peekIdempotent(l.ctx, l.svcCtx.DB, endpointTransitionOrderItem,
		in.GetVenueId(), in.GetIdempotencyKey(), fingerprint, func() *v1_orderpb.Order { return &v1_orderpb.Order{} }); err != nil {
		return nil, err
	} else if found {
		return replay, nil
	}

	return runIdempotent(l.ctx, l.svcCtx.DB, endpointTransitionOrderItem,
		in.GetVenueId(), in.GetIdempotencyKey(), fingerprint,
		func() *v1_orderpb.Order { return &v1_orderpb.Order{} },
		func(ctx context.Context, s sqlx.Session) (*v1_orderpb.Order, error) {
			return l.apply(ctx, s, in, to)
		})
}

func (l *TransitionOrderItemLogic) apply(
	ctx context.Context,
	s sqlx.Session,
	in *v1_orderpb.TransitionOrderItemRequest,
	to string,
) (*v1_orderpb.Order, error) {
	orders := model.NewOrderModel(s)

	// Lock the parent order, not the line: the FR-K5 rule below reads
	// every sibling line's status, so the whole aggregate has to be
	// stable for the duration. Locking the order also serializes two
	// staff transitioning two *different* lines of the same ticket, which
	// is exactly when the auto-ready rule would otherwise race.
	order, err := orders.FindByIDForUpdate(ctx, in.GetVenueId(), in.GetOrderId())
	if err != nil {
		if isNotFound(err) {
			return nil, apierr.NotFound("order not found")
		}
		l.Errorf("lock order %s: %v", in.GetOrderId(), err)
		return nil, apierr.Internal("look up order")
	}

	item, err := orders.FindItem(ctx, order.ID, in.GetOrderItemId())
	if err != nil {
		if isNotFound(err) {
			return nil, apierr.NotFound("order item not found")
		}
		l.Errorf("look up order item %s: %v", in.GetOrderItemId(), err)
		return nil, apierr.Internal("look up order item")
	}

	if item.Status == to {
		return l.reload(ctx, orders, in.GetVenueId(), order.ID)
	}
	if !canTransitionItem(item.Status, to) {
		return nil, apierr.InvalidTransition(fmt.Sprintf("cannot move an order item from %s to %s", item.Status, to))
	}
	if isTerminalOrder(order.Status) {
		return nil, apierr.InvalidTransition(fmt.Sprintf("order is %s; its items can no longer change", order.Status))
	}

	if _, err := orders.UpdateItemStatus(ctx, order.ID, item.ID, to, item.Status); err != nil {
		if isNotFound(err) {
			return nil, apierr.InvalidTransition("the order item changed while this request was in flight")
		}
		l.Errorf("transition order item %s: %v", item.ID, err)
		return nil, apierr.Internal("transition order item")
	}

	// Re-price the order from its live lines. Done unconditionally rather
	// than only on cancel: it is one cheap aggregate, and deriving the
	// total from the lines can't drift the way an incremental adjustment
	// can.
	newTotal, err := orders.LiveTotal(ctx, order.ID)
	if err != nil {
		l.Errorf("recompute order total for %s: %v", order.ID, err)
		return nil, apierr.Internal("recompute order total")
	}
	if newTotal != order.TotalMinor {
		if err := orders.SetTotal(ctx, order.ID, newTotal); err != nil {
			l.Errorf("update order total for %s: %v", order.ID, err)
			return nil, apierr.Internal("update order total")
		}
		if err := model.NewTableSessionModel(s).AddToTotal(ctx, order.TableSessionID, newTotal-order.TotalMinor); err != nil {
			l.Errorf("adjust table session total: %v", err)
			return nil, apierr.Internal("adjust table session total")
		}
	}

	// FR-K5 and its mirror: derive whether the order itself should move.
	orderStatus := order.Status
	counts, err := orders.ItemStatusCounts(ctx, order.ID)
	if err != nil {
		l.Errorf("count item statuses for %s: %v", order.ID, err)
		return nil, apierr.Internal("count item statuses")
	}
	if derived := deriveOrderStatus(order.Status, counts); derived != "" {
		if _, err := orders.UpdateStatus(ctx, in.GetVenueId(), order.ID, derived, order.Status, ""); err != nil {
			l.Errorf("auto-transition order %s to %s: %v", order.ID, derived, err)
			return nil, apierr.Internal("auto-transition order")
		}
		orderStatus = derived

		if derived == model.OrderCancelled {
			// Every line was cancelled, so the order contributes nothing;
			// LiveTotal above already returned 0 and the session total was
			// adjusted with it. Nothing further to correct here.
			l.Infof("order %s auto-cancelled: all items cancelled", order.ID)
		}

		eventType := eventOrderTransitioned
		if derived == model.OrderCancelled {
			eventType = eventOrderCancelled
		}
		if err := model.NewOutboxModel(s).Insert(ctx, eventType, in.GetVenueId(), model.TopicOrder, order.ID,
			orderTransitionedPayload{
				OrderID:        order.ID,
				Number:         order.Number,
				TableID:        order.TableID,
				TableSessionID: order.TableSessionID,
				From:           order.Status,
				To:             derived,
				Reason:         "derived from item statuses",
				ActorStaffID:   in.GetActorStaffId(),
				TotalMinor:     newTotal,
				Currency:       order.Currency,
			}, traceID(ctx)); err != nil {
			l.Errorf("write derived %s outbox row: %v", eventType, err)
			return nil, apierr.Internal("write outbox row")
		}
	}

	if err := model.NewOutboxModel(s).Insert(ctx, eventOrderItemTransitioned, in.GetVenueId(),
		model.TopicOrder, order.ID,
		orderItemTransitionedPayload{
			OrderID:      order.ID,
			OrderItemID:  item.ID,
			MenuItemID:   item.MenuItemID,
			TableID:      order.TableID,
			From:         item.Status,
			To:           to,
			Reason:       in.GetReason(),
			ActorStaffID: in.GetActorStaffId(),
			OrderStatus:  orderStatus,
		}, traceID(ctx)); err != nil {
		l.Errorf("write order.item_transitioned outbox row: %v", err)
		return nil, apierr.Internal("write outbox row")
	}

	return l.reload(ctx, orders, in.GetVenueId(), order.ID)
}

func (l *TransitionOrderItemLogic) reload(ctx context.Context, orders *model.OrderModel, venueID, orderID string) (*v1_orderpb.Order, error) {
	full, err := orders.FindByID(ctx, venueID, orderID)
	if err != nil {
		l.Errorf("reload order %s: %v", orderID, err)
		return nil, apierr.Internal("reload order")
	}
	return orderToProto(full), nil
}
