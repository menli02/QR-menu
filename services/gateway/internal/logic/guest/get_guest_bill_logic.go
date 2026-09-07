// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package guest

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

type GetGuestBillLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetGuestBillLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetGuestBillLogic {
	return &GetGuestBillLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// GetGuestBill returns the current table session's bill (FR-S5).
//
// The session is looked up from the guest's own table rather than taken as
// a parameter, so there is no id for a guest to substitute. §8.1 gives
// this route no path parameter at all, which is the same decision
// expressed in the URL.
func (l *GetGuestBillLogic) GetGuestBill() (resp *types.Bill, err error) {
	claims, err := authz.Guest(l.ctx)
	if err != nil {
		return nil, err
	}

	session, err := l.svcCtx.OrderRpc.GetTableSession(l.ctx, &v1_orderpb.GetTableSessionRequest{
		VenueId: claims.VenueID,
		TableId: claims.TableID,
	})
	if err != nil {
		converted := rpcerr.From(err)
		if converted.Code == errs.CodeNotFound {
			return nil, errs.New(errs.CodeNotFound, "nothing has been ordered at this table yet")
		}
		return nil, converted
	}

	bill, err := l.svcCtx.OrderRpc.GetBill(l.ctx, &v1_orderpb.GetBillRequest{
		VenueId:        claims.VenueID,
		TableSessionId: session.GetId(),
	})
	if err != nil {
		return nil, rpcerr.From(err)
	}

	out := convert.Bill(bill)
	return &out, nil
}
