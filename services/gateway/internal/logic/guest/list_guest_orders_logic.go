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

type ListGuestOrdersLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewListGuestOrdersLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListGuestOrdersLogic {
	return &ListGuestOrdersLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// ListGuestOrders shows the table session's orders and their live statuses
// (FR-O10).
//
// It is the whole *table's* orders, not just this guest's: FR-O9 has
// everyone at a table sharing one session, and FR-O10 promises a running
// table total, so a party that ordered from three phones has to see one
// list. convert.Order drops guest_session_id from the public shape so this
// still doesn't leak who ordered what between strangers seated together.
//
// It is also the guest's polling fallback when the WebSocket is
// unavailable (§7.3), which is why it stays a plain, cheap read.
func (l *ListGuestOrdersLogic) ListGuestOrders() (resp *types.ListGuestOrdersResp, err error) {
	claims, err := authz.Guest(l.ctx)
	if err != nil {
		return nil, err
	}

	session, err := l.currentSession(claims.VenueID, claims.TableID)
	if err != nil {
		return nil, err
	}
	if session == nil {
		// No session open at this table yet — the guest scanned the code
		// but hasn't ordered. An empty list is the truthful answer, and
		// far better for the client than a 404 it has to special-case.
		return &types.ListGuestOrdersResp{Orders: []types.Order{}}, nil
	}

	orders, err := l.svcCtx.OrderRpc.ListOrders(l.ctx, &v1_orderpb.ListOrdersRequest{
		VenueId:        claims.VenueID,
		TableSessionId: session.GetId(),
	})
	if err != nil {
		return nil, rpcerr.From(err)
	}

	return &types.ListGuestOrdersResp{Orders: convert.Orders(orders.GetOrders())}, nil
}

// currentSession returns the open session at the guest's table, or nil if
// there isn't one. Shared with the bill handler.
func (l *ListGuestOrdersLogic) currentSession(venueID, tableID string) (*v1_orderpb.TableSession, error) {
	session, err := l.svcCtx.OrderRpc.GetTableSession(l.ctx, &v1_orderpb.GetTableSessionRequest{
		VenueId: venueID,
		TableId: tableID,
	})
	if err != nil {
		converted := rpcerr.From(err)
		if converted.Code == errs.CodeNotFound {
			return nil, nil
		}
		return nil, converted
	}
	return session, nil
}
