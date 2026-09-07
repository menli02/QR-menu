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

type CloseTableSessionLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewCloseTableSessionLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CloseTableSessionLogic {
	return &CloseTableSessionLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// CloseTableSession marks the bill paid and archives the session (FR-S6).
// R1 records the method only, not the amount received or change given.
//
// One call closes the session, revokes the party's guest tokens and
// resolves anything still open on the table — all in one transaction in
// the order service, so a half-closed table is not a state that exists.
//
// A repeat gets SESSION_CLOSED rather than a fabricated success. That is
// the right answer for this action specifically: a second close usually
// means two staff are both settling the same table, and the second one
// needs to know the first already took the payment.
func (l *CloseTableSessionLogic) CloseTableSession(req *types.CloseTableSessionReq) (resp *types.TableSession, err error) {
	claims, err := authz.Staff(l.ctx)
	if err != nil {
		return nil, err
	}
	if req.Id == "" {
		return nil, errs.New(errs.CodeValidationFailed, "table session id is required")
	}

	method, ok := convert.PaymentMethods[req.PaymentMethod]
	if !ok {
		return nil, errs.New(errs.CodeValidationFailed,
			"paymentMethod must be cash, card_terminal or other")
	}

	session, err := l.svcCtx.OrderRpc.CloseTableSession(l.ctx, &v1_orderpb.CloseTableSessionRequest{
		VenueId:        claims.VenueID,
		TableSessionId: req.Id,
		PaymentMethod:  method,
		ActorStaffId:   claims.StaffID,
	})
	if err != nil {
		return nil, rpcerr.From(err)
	}

	out := convert.TableSession(session)
	return &out, nil
}
