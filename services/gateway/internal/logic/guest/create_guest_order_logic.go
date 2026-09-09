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

type CreateGuestOrderLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewCreateGuestOrderLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateGuestOrderLogic {
	return &CreateGuestOrderLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// CreateGuestOrder places an order (FR-O3..O9).
//
// Everything that identifies *who and where* — venue, table, guest session
// — comes from the verified token, never the body. The request carries
// only what the guest legitimately chooses: items, quantities, modifiers
// and comments. That is what makes it structurally impossible to order
// onto another table's bill.
//
// Per-line prices are absent from the request by design (FR-O3): the
// server re-prices every line against the live menu, and the client's
// displayed total is never trusted.
//
// expectedTotalMinor is not an exception to that. It is not used to
// charge anything — it is compared against the freshly resolved total,
// and a mismatch fails the submit with PRICE_CHANGED (FR-O7) so the guest
// re-confirms rather than paying a price they never saw.
func (l *CreateGuestOrderLogic) CreateGuestOrder(req *types.CreateGuestOrderReq) (resp *types.Order, err error) {
	claims, err := authz.Guest(l.ctx)
	if err != nil {
		return nil, err
	}

	key := reqctx.IdempotencyKey(l.ctx)
	if key == "" {
		return nil, errs.New(errs.CodeValidationFailed,
			"an Idempotency-Key header is required when placing an order")
	}
	if len(req.Items) == 0 {
		return nil, errs.New(errs.CodeValidationFailed, "an order must contain at least one item")
	}

	items := make([]*v1_orderpb.OrderItemRequest, 0, len(req.Items))
	for _, it := range req.Items {
		items = append(items, &v1_orderpb.OrderItemRequest{
			ItemId:            it.ItemId,
			Qty:               it.Qty,
			ModifierOptionIds: it.ModifierOptionIds,
			Comment:           it.Comment,
		})
	}

	// The order service owns the rest of FR-O5's validation (quantities,
	// comment length, line count, venue total limit). Duplicating those
	// bounds here would be a second source of truth for the same rules;
	// the gateway checks only what it can see without a round trip.
	order, err := l.svcCtx.OrderRpc.CreateOrder(l.ctx, &v1_orderpb.CreateOrderRequest{
		VenueId:        claims.VenueID,
		TableId:        claims.TableID,
		GuestSessionId: claims.GuestSessionID,
		IdempotencyKey: key,
		Items:          items,
		// FR-O7. Zero means the client did not say what it was showing,
		// which the order service reads as "no expectation" and skips the
		// check — so an older client keeps working without it.
		ExpectedTotalMinor: req.ExpectedTotalMinor,
	})
	if err != nil {
		return nil, rpcerr.From(err)
	}

	out := convert.Order(order)
	return &out, nil
}
