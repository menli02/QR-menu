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

type TransitionOrderItemLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewTransitionOrderItemLogic(ctx context.Context, svcCtx *svc.ServiceContext) *TransitionOrderItemLogic {
	return &TransitionOrderItemLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// TransitionOrderItem is the per-line KDS action (FR-K3): cooking, ready,
// cancel.
//
// It returns the whole order, not just the line, because a line change can
// move the order with it — FR-K5 promotes an order to ready once its last
// live line is ready. The KDS needs both facts from one response or its
// ticket will show a stale header.
func (l *TransitionOrderItemLogic) TransitionOrderItem(req *types.TransitionOrderItemReq) (resp *types.Order, err error) {
	claims, err := authz.Staff(l.ctx)
	if err != nil {
		return nil, err
	}
	if req.Id == "" || req.OrderId == "" {
		return nil, errs.New(errs.CodeValidationFailed, "order item id and orderId are required")
	}

	to, ok := convert.OrderItemStatuses[req.To]
	if !ok {
		return nil, errs.New(errs.CodeValidationFailed, "to must be one of cooking, ready, cancelled")
	}

	key := reqctx.IdempotencyKey(l.ctx)
	if key == "" {
		return nil, errs.New(errs.CodeValidationFailed,
			"an Idempotency-Key header is required for this action")
	}

	order, err := l.svcCtx.OrderRpc.TransitionOrderItem(l.ctx, &v1_orderpb.TransitionOrderItemRequest{
		VenueId:        claims.VenueID,
		OrderId:        req.OrderId,
		OrderItemId:    req.Id,
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
