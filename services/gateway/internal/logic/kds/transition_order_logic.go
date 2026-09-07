// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package kds

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

type TransitionOrderLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewTransitionOrderLogic(ctx context.Context, svcCtx *svc.ServiceContext) *TransitionOrderLogic {
	return &TransitionOrderLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// TransitionOrder is the KDS ticket action (FR-K3): accept, start, ready,
// served, cancel.
//
// actor_staff_id is always set from the verified token, which does double
// duty: it records who acted for FR-A4's audit trail, and it is how the
// order service tells a staff action from a guest self-cancel (an empty
// actor means the guest). A staff request must therefore never reach it
// with an empty actor, or a waiter's cancellation would be silently
// subjected to the guest cancel-window rule.
//
// The idempotency key matters more here than it looks: FR-K7 has offline
// KDS clients queueing actions and replaying them on reconnect, so the
// same "mark ready" can legitimately arrive twice.
func (l *TransitionOrderLogic) TransitionOrder(req *types.TransitionOrderReq) (resp *types.Order, err error) {
	claims, err := authz.Staff(l.ctx)
	if err != nil {
		return nil, err
	}
	if req.Id == "" {
		return nil, errs.New(errs.CodeValidationFailed, "order id is required")
	}

	to, ok := convert.OrderStatuses[req.To]
	if !ok {
		return nil, errs.New(errs.CodeValidationFailed,
			"to must be one of accepted, in_progress, ready, served, cancelled")
	}

	key := reqctx.IdempotencyKey(l.ctx)
	if key == "" {
		return nil, errs.New(errs.CodeValidationFailed,
			"an Idempotency-Key header is required for this action")
	}

	order, err := l.svcCtx.OrderRpc.TransitionOrder(l.ctx, &v1_orderpb.TransitionOrderRequest{
		VenueId:        claims.VenueID,
		OrderId:        req.Id,
		To:             to,
		Reason:         req.Reason,
		ActorStaffId:   claims.StaffID,
		IdempotencyKey: key,
	})
	if err != nil {
		return nil, rpcerr.From(err)
	}

	out := convert.Order(order)
	return &out, nil
}
