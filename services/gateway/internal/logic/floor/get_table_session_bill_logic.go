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

type GetTableSessionBillLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetTableSessionBillLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetTableSessionBillLogic {
	return &GetTableSessionBillLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// GetTableSessionBill is the staff-side bill for any table (FR-S5).
//
// Unlike the guest route, this one takes a session id — a waiter carrying
// a card machine legitimately needs to pull up whichever table they are
// standing at. The venue scoping still comes from the token, so a staff
// member cannot read a session belonging to another venue: order's own
// lookup filters on venue_id and returns NOT_FOUND rather than the bill.
func (l *GetTableSessionBillLogic) GetTableSessionBill(req *types.GetTableSessionBillReq) (resp *types.Bill, err error) {
	claims, err := authz.Staff(l.ctx)
	if err != nil {
		return nil, err
	}
	if req.Id == "" {
		return nil, errs.New(errs.CodeValidationFailed, "table session id is required")
	}

	bill, err := l.svcCtx.OrderRpc.GetBill(l.ctx, &v1_orderpb.GetBillRequest{
		VenueId:        claims.VenueID,
		TableSessionId: req.Id,
	})
	if err != nil {
		return nil, rpcerr.From(err)
	}

	out := convert.Bill(bill)
	return &out, nil
}
