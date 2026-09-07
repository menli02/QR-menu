package orderservicelogic

import (
	"context"
	"fmt"
	"time"

	"github.com/menli02/QR-menu/proto/order/v1"
	"github.com/menli02/QR-menu/services/order/internal/apierr"
	"github.com/menli02/QR-menu/services/order/internal/model"
	"github.com/menli02/QR-menu/services/order/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

type TransitionOrderLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewTransitionOrderLogic(ctx context.Context, svcCtx *svc.ServiceContext) *TransitionOrderLogic {
	return &TransitionOrderLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// TransitionOrder moves a whole ticket through the kitchen state machine
// (FR-K3) and is also the guest's self-cancel path (FR-O11).
//
// Authorisation boundary worth being explicit about: an empty
// actor_staff_id means "the guest acted", per the field's own proto
// comment, and that is all this service can know. The request carries no
// guest_session_id, so **the order service cannot verify that the
// cancelling guest owns this order** — the gateway does that, comparing
// the order's guest_session_id against the guest JWT before it ever calls
// here. What this service enforces is the part it can: that a guest
// cancel is only ever a cancel, only from `placed`, and only inside the
// venue's cancel window.
func (l *TransitionOrderLogic) TransitionOrder(in *v1_orderpb.TransitionOrderRequest) (*v1_orderpb.Order, error) {
	if in.GetVenueId() == "" || in.GetOrderId() == "" {
		return nil, apierr.Validation("venue_id and order_id are required")
	}
	to, ok := orderStatusFromProto[in.GetTo()]
	if !ok {
		return nil, apierr.Validation("to must be a known order status")
	}

	settings, err := l.svcCtx.Venue.Get(l.ctx, in.GetVenueId())
	if err != nil {
		return nil, catalogError(err, "load venue settings")
	}

	fingerprint := model.Fingerprint(in.GetVenueId(), in.GetOrderId(), to, in.GetActorStaffId())

	if replay, found, err := peekIdempotent(l.ctx, l.svcCtx.DB, endpointTransitionOrder,
		in.GetVenueId(), in.GetIdempotencyKey(), fingerprint, func() *v1_orderpb.Order { return &v1_orderpb.Order{} }); err != nil {
		return nil, err
	} else if found {
		return replay, nil
	}

	return runIdempotent(l.ctx, l.svcCtx.DB, endpointTransitionOrder,
		in.GetVenueId(), in.GetIdempotencyKey(), fingerprint,
		func() *v1_orderpb.Order { return &v1_orderpb.Order{} },
		func(ctx context.Context, s sqlx.Session) (*v1_orderpb.Order, error) {
			return l.apply(ctx, s, in, to, settings.CancelWindow)
		})
}

func (l *TransitionOrderLogic) apply(
	ctx context.Context,
	s sqlx.Session,
	in *v1_orderpb.TransitionOrderRequest,
	to string,
	cancelWindow time.Duration,
) (*v1_orderpb.Order, error) {
	orders := model.NewOrderModel(s)

	// Lock the row first: two KDS tablets pressing the same button at the
	// same moment must serialize here, not both read 'accepted' and both
	// try to write.
	current, err := orders.FindByIDForUpdate(ctx, in.GetVenueId(), in.GetOrderId())
	if err != nil {
		if isNotFound(err) {
			return nil, apierr.NotFound("order not found")
		}
		l.Errorf("lock order %s: %v", in.GetOrderId(), err)
		return nil, apierr.Internal("look up order")
	}

	// A retry that arrives after the transition already landed — a
	// different idempotency key, or none — is a success, not a conflict.
	// The kitchen pressed "ready"; the order is ready.
	if current.Status == to {
		full, err := orders.FindByID(ctx, in.GetVenueId(), in.GetOrderId())
		if err != nil {
			return nil, apierr.Internal("reload order")
		}
		return orderToProto(full), nil
	}

	if !canTransitionOrder(current.Status, to) {
		return nil, apierr.InvalidTransition(fmt.Sprintf("cannot move an order from %s to %s", current.Status, to))
	}

	isGuestAction := in.GetActorStaffId() == ""
	if isGuestAction {
		if to != model.OrderCancelled {
			return nil, apierr.InvalidTransition("a guest may only cancel an order")
		}
		if current.Status != model.OrderPlaced {
			return nil, apierr.InvalidTransition("the kitchen has already started this order")
		}
		if elapsed := time.Since(current.PlacedAt); elapsed > cancelWindow {
			return nil, apierr.InvalidTransition(fmt.Sprintf(
				"the %s cancellation window for this order has passed", cancelWindow))
		}
	}

	updated, err := orders.UpdateStatus(ctx, in.GetVenueId(), in.GetOrderId(), to, current.Status, in.GetReason())
	if err != nil {
		if isNotFound(err) {
			// Unreachable while we hold the row lock, but a lost
			// compare-and-set is exactly the bug worth reporting honestly
			// rather than papering over.
			return nil, apierr.InvalidTransition("the order changed while this request was in flight")
		}
		l.Errorf("transition order %s: %v", in.GetOrderId(), err)
		return nil, apierr.Internal("transition order")
	}

	// Cancelling a whole order zeroes its contribution to the table
	// session total; the lines stay on the record for the day report and
	// for the guest's history.
	if to == model.OrderCancelled {
		if err := model.NewTableSessionModel(s).AddToTotal(ctx, updated.TableSessionID, -current.TotalMinor); err != nil {
			l.Errorf("adjust table session total after cancel: %v", err)
			return nil, apierr.Internal("adjust table session total")
		}
	}

	eventType := eventOrderTransitioned
	if to == model.OrderCancelled {
		// §8.3 lists order.cancelled as its own R1 event type. Consumers
		// that only care about cancellations shouldn't have to subscribe
		// to every transition and filter.
		eventType = eventOrderCancelled
	}
	if err := model.NewOutboxModel(s).Insert(ctx, eventType, in.GetVenueId(), model.TopicOrder, updated.ID,
		orderTransitionedPayload{
			OrderID:        updated.ID,
			Number:         updated.Number,
			TableID:        updated.TableID,
			TableSessionID: updated.TableSessionID,
			From:           current.Status,
			To:             to,
			Reason:         in.GetReason(),
			ActorStaffID:   in.GetActorStaffId(),
			TotalMinor:     updated.TotalMinor,
			Currency:       updated.Currency,
		}, traceID(ctx)); err != nil {
		l.Errorf("write %s outbox row: %v", eventType, err)
		return nil, apierr.Internal("write outbox row")
	}

	full, err := orders.FindByID(ctx, in.GetVenueId(), in.GetOrderId())
	if err != nil {
		l.Errorf("reload order %s: %v", in.GetOrderId(), err)
		return nil, apierr.Internal("reload order")
	}
	return orderToProto(full), nil
}
