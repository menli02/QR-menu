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

type CreateGuestServiceRequestLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewCreateGuestServiceRequestLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateGuestServiceRequestLogic {
	return &CreateGuestServiceRequestLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// CreateGuestServiceRequest is FR-S1: call a waiter, or ask for the bill.
//
// No table session is passed. A guest may call a waiter before ordering
// anything, so the order service opens-or-joins one itself — the same path
// CreateOrder takes.
//
// FR-S2's "one open request per type per table" is enforced there too, as
// a partial unique index rather than a timer, and a repeat returns the
// existing request instead of an error. Tapping the button twice is not a
// mistake worth reporting to a guest.
func (l *CreateGuestServiceRequestLogic) CreateGuestServiceRequest(req *types.CreateGuestServiceRequestReq) (resp *types.ServiceRequest, err error) {
	claims, err := authz.Guest(l.ctx)
	if err != nil {
		return nil, err
	}

	reqType, ok := convert.ServiceRequestTypes[req.Type]
	if !ok {
		return nil, errs.New(errs.CodeValidationFailed, "type must be call_waiter or request_bill")
	}

	key := reqctx.IdempotencyKey(l.ctx)
	if key == "" {
		return nil, errs.New(errs.CodeValidationFailed,
			"an Idempotency-Key header is required for this request")
	}

	sr, err := l.svcCtx.OrderRpc.CreateServiceRequest(l.ctx, &v1_orderpb.CreateServiceRequestRequest{
		VenueId:        claims.VenueID,
		TableId:        claims.TableID,
		Type:           reqType,
		Note:           req.Note,
		IdempotencyKey: key,
	})
	if err != nil {
		return nil, rpcerr.From(err)
	}

	out := convert.ServiceRequest(sr)
	return &out, nil
}
