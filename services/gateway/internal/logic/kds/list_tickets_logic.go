// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package kds

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

// ticketPageSize is what one KDS screen asks for. The route's public
// shape (§8.1) has no pageSize parameter — a kitchen display shows the
// active queue, it doesn't page — so the gateway picks it.
const ticketPageSize = 200

type ListTicketsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewListTicketsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListTicketsLogic {
	return &ListTicketsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// ListTickets is the KDS queue (FR-K1), oldest first.
//
// `status=active` (the default) leaves the filter empty and lets the order
// service apply its own definition of active — placed through ready.
// Keeping that definition in one place means adding a state later doesn't
// require remembering to update the gateway too.
//
// FR-K4's colour escalation is not computed here: the thresholds are venue
// settings the KDS already holds, every ticket carries placedAt, and a
// server-rendered colour would be stale before it reached the screen.
func (l *ListTicketsLogic) ListTickets(req *types.ListTicketsReq) (resp *types.ListTicketsResp, err error) {
	claims, err := authz.Staff(l.ctx)
	if err != nil {
		return nil, err
	}

	var filter []v1_orderpb.OrderStatus
	switch req.Status {
	case "", "active":
		// Empty filter: the order service reads this as its active subset.
	case "all":
		for _, s := range convert.OrderStatuses {
			filter = append(filter, s)
		}
	default:
		return nil, errs.New(errs.CodeValidationFailed, "status must be active or all")
	}

	tickets, err := l.svcCtx.OrderRpc.ListTickets(l.ctx, &v1_orderpb.ListTicketsRequest{
		VenueId:      claims.VenueID,
		StatusFilter: filter,
		PageSize:     ticketPageSize,
	})
	if err != nil {
		return nil, rpcerr.From(err)
	}

	return &types.ListTicketsResp{Orders: convert.Orders(tickets.GetOrders())}, nil
}
