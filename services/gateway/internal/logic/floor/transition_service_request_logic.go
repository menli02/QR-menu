// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package floor

import (
	"context"

	v1_orderpb "github.com/menli02/QR-menu/proto/order/v1"
	"github.com/menli02/QR-menu/services/gateway/internal/authz"
	"github.com/menli02/QR-menu/services/gateway/internal/convert"
	"github.com/menli02/QR-menu/services/gateway/internal/errs"
	"github.com/menli02/QR-menu/services/gateway/internal/rpcerr"
	"github.com/menli02/QR-menu/services/gateway/internal/svc"
	"github.com/menli02/QR-menu/services/gateway/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type TransitionServiceRequestLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewTransitionServiceRequestLogic(ctx context.Context, svcCtx *svc.ServiceContext) *TransitionServiceRequestLogic {
	return &TransitionServiceRequestLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// TransitionServiceRequest acknowledges or resolves a guest request
// (FR-S3).
//
// Only those two targets are accepted. `expired` belongs to FR-S4's
// automatic sweep, and `open` is where a request starts — neither is
// something a waiter should be able to set by hand, which would let a
// request be quietly un-acknowledged after the fact.
//
// No idempotency key: the order service applies this as a compare-and-set
// and reports a repeat of a transition that already landed as success.
// Two waiters both tapping "acknowledge" is normal, and neither should see
// an error.
func (l *TransitionServiceRequestLogic) TransitionServiceRequest(req *types.TransitionServiceRequestReq) (resp *types.ServiceRequest, err error) {
	claims, err := authz.Staff(l.ctx)
	if err != nil {
		return nil, err
	}
	if req.Id == "" {
		return nil, errs.New(errs.CodeValidationFailed, "service request id is required")
	}

	to, ok := convert.ServiceRequestStatuses[req.To]
	if !ok || (req.To != "acknowledged" && req.To != "resolved") {
		return nil, errs.New(errs.CodeValidationFailed, "to must be acknowledged or resolved")
	}

	sr, err := l.svcCtx.OrderRpc.TransitionServiceRequest(l.ctx, &v1_orderpb.TransitionServiceRequestRequest{
		VenueId:          claims.VenueID,
		ServiceRequestId: req.Id,
		To:               to,
		ActorStaffId:     claims.StaffID,
	})
	if err != nil {
		return nil, rpcerr.From(err)
	}

	out := convert.ServiceRequest(sr)
	return &out, nil
}
