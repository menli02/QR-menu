// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package guest

import (
	"context"

	v1_orderpb "github.com/menli02/QR-menu/proto/order/v1"
	"github.com/menli02/QR-menu/services/gateway/internal/authz"
	"github.com/menli02/QR-menu/services/gateway/internal/convert"
	"github.com/menli02/QR-menu/services/gateway/internal/errs"
	"github.com/menli02/QR-menu/services/gateway/internal/reqctx"
	"github.com/menli02/QR-menu/services/gateway/internal/rpcerr"
	"github.com/menli02/QR-menu/services/gateway/internal/svc"
	"github.com/menli02/QR-menu/services/gateway/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type CancelGuestOrderLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewCancelGuestOrderLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CancelGuestOrderLogic {
	return &CancelGuestOrderLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// CancelGuestOrder is FR-O11: a guest may cancel their *own* order inside
// the venue's cancel window, while it is still `placed`.
//
// Ownership is enforced here, and it has to be. TransitionOrderRequest
// carries no guest_session_id — an empty actor_staff_id is all the order
// service sees, and it reads that as "a guest acted", not as "this
// specific guest acted". So the gateway is the only component holding both
// the verified guest identity and the order's, and it checks them.
// Without this read-before-write, any guest at any table could cancel any
// order in the venue by id.
//
// The window and the status check stay in the order service, where they
// are enforced inside the transaction that does the write.
func (l *CancelGuestOrderLogic) CancelGuestOrder(req *types.CancelGuestOrderReq) (resp *types.Order, err error) {
	claims, err := authz.Guest(l.ctx)
	if err != nil {
		return nil, err
	}
	if req.Id == "" {
		return nil, errs.New(errs.CodeValidationFailed, "order id is required")
	}

	key := reqctx.IdempotencyKey(l.ctx)
	if key == "" {
		return nil, errs.New(errs.CodeValidationFailed,
			"an Idempotency-Key header is required when cancelling an order")
	}

	if err := l.assertOwnedByGuest(claims.VenueID, claims.TableID, claims.GuestSessionID, req.Id); err != nil {
		return nil, err
	}

	order, err := l.svcCtx.OrderRpc.TransitionOrder(l.ctx, &v1_orderpb.TransitionOrderRequest{
		VenueId:        claims.VenueID,
		OrderId:        req.Id,
		To:             v1_orderpb.OrderStatus_ORDER_STATUS_CANCELLED,
		Reason:         req.Reason,
		ActorStaffId:   "", // empty = the guest acted (FR-O11)
		IdempotencyKey: key,
	})
	if err != nil {
		return nil, rpcerr.From(err)
	}

	out := convert.Order(order)
	return &out, nil
}

// assertOwnedByGuest confirms the order belongs to this guest's session.
//
// It reads the table's order list rather than fetching one order by id,
// because that is the only order RPC scoped to a table session — and the
// scoping is the point: an order id from another table simply will not
// appear, so it reports NOT_FOUND without ever revealing that the id is
// real.
func (l *CancelGuestOrderLogic) assertOwnedByGuest(venueID, tableID, guestSessionID, orderID string) error {
	session, err := l.svcCtx.OrderRpc.GetTableSession(l.ctx, &v1_orderpb.GetTableSessionRequest{
		VenueId: venueID,
		TableId: tableID,
	})
	if err != nil {
		converted := rpcerr.From(err)
		if converted.Code == errs.CodeNotFound {
			return errs.New(errs.CodeNotFound, "order not found")
		}
		return converted
	}

	orders, err := l.svcCtx.OrderRpc.ListOrders(l.ctx, &v1_orderpb.ListOrdersRequest{
		VenueId:        venueID,
		TableSessionId: session.GetId(),
	})
	if err != nil {
		return rpcerr.From(err)
	}

	for _, o := range orders.GetOrders() {
		if o.GetId() != orderID {
			continue
		}
		if o.GetGuestSessionId() != guestSessionID {
			// The order is real and at this table, but belongs to another
			// guest in the party. FORBIDDEN rather than NOT_FOUND: they
			// can already see it in their own order list, so hiding it
			// would only be confusing.
			return errs.New(errs.CodeForbidden, "this order was placed by someone else at your table")
		}
		return nil
	}
	return errs.New(errs.CodeNotFound, "order not found")
}
