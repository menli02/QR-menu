package orderservicelogic

import (
	"context"

	"github.com/menli02/QR-menu/proto/order/v1"
	"github.com/menli02/QR-menu/services/order/internal/apierr"
	"github.com/menli02/QR-menu/services/order/internal/model"
	"github.com/menli02/QR-menu/services/order/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetBillLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetBillLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetBillLogic {
	return &GetBillLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// GetBill computes the bill for a table session (FR-S5): every line
// across every order on the session, plus the venue's service charge.
//
// It is computed fresh from `orders` on every call rather than read from
// table_sessions.total_minor. That column is a denormalized mirror for
// the floor view's at-a-glance totals; this is the number a guest pays,
// so it comes from the source of truth. If the two ever disagree, the
// bill is right and the mirror has a bug.
func (l *GetBillLogic) GetBill(in *v1_orderpb.GetBillRequest) (*v1_orderpb.Bill, error) {
	if in.GetVenueId() == "" || in.GetTableSessionId() == "" {
		return nil, apierr.Validation("venue_id and table_session_id are required")
	}

	session, err := model.NewTableSessionModel(l.svcCtx.DB).FindByID(l.ctx, in.GetVenueId(), in.GetTableSessionId())
	if err != nil {
		if isNotFound(err) {
			return nil, apierr.NotFound("table session not found")
		}
		l.Errorf("look up table session %s: %v", in.GetTableSessionId(), err)
		return nil, apierr.Internal("look up table session")
	}

	orders, err := model.NewOrderModel(l.svcCtx.DB).ListByTableSession(l.ctx, in.GetVenueId(), in.GetTableSessionId())
	if err != nil {
		l.Errorf("list orders for bill %s: %v", in.GetTableSessionId(), err)
		return nil, apierr.Internal("list orders")
	}

	settings, err := l.svcCtx.Venue.Get(l.ctx, in.GetVenueId())
	if err != nil {
		return nil, catalogError(err, "load venue settings")
	}

	bill := &v1_orderpb.Bill{
		TableSessionId:   session.ID,
		VenueId:          session.VenueID,
		Currency:         session.Currency,
		ServiceChargeBps: settings.ServiceChargeBps,
	}

	for i := range orders {
		o := &orders[i]
		// A cancelled order is not billed, and neither is a cancelled
		// line of a live order.
		if o.Status == model.OrderCancelled {
			continue
		}
		for j := range o.Items {
			it := &o.Items[j]
			if it.Status == model.ItemCancelled {
				continue
			}
			bill.LineItems = append(bill.LineItems, &v1_orderpb.BillLineItem{
				OrderId:        o.ID,
				MenuItemId:     it.MenuItemID,
				Name:           it.Name,
				Qty:            it.Qty,
				UnitPrice:      money(it.UnitPriceMinor, session.Currency),
				LineTotalMinor: it.LineTotalMinor,
			})
			bill.SubtotalMinor += it.LineTotalMinor
		}
	}

	bill.ServiceChargeMinor = serviceCharge(bill.SubtotalMinor, settings.ServiceChargeBps)
	bill.TotalMinor = bill.SubtotalMinor + bill.ServiceChargeMinor
	return bill, nil
}

// serviceCharge applies a basis-point rate to a minor-unit amount,
// rounding half up.
//
// Rounding is spelled out rather than left to integer truncation because
// it is money: on a 10% charge over a 1,234-unit subtotal, truncation
// quietly under-bills by one unit every time, and "the till is a penny
// short, every night" is a genuinely awful bug to chase.
func serviceCharge(subtotalMinor int64, bps int32) int64 {
	if bps <= 0 || subtotalMinor <= 0 {
		return 0
	}
	return (subtotalMinor*int64(bps) + 5000) / 10000
}
