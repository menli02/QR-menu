package orderservicelogic

import (
	"context"
	"time"

	"github.com/menli02/QR-menu/proto/order/v1"
	"github.com/menli02/QR-menu/services/order/internal/apierr"
	"github.com/menli02/QR-menu/services/order/internal/model"
	"github.com/menli02/QR-menu/services/order/internal/svc"
	"github.com/menli02/QR-menu/services/order/internal/venue"

	"github.com/zeromicro/go-zero/core/logx"
)

// topItemsLimit bounds the best-sellers list. Ten is what fits on an
// admin dashboard card; the proto puts no cap on the repeated field, so
// the limit lives here rather than being left to the client.
const topItemsLimit = 10

type GetDayReportLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetDayReportLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetDayReportLogic {
	return &GetDayReportLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// GetDayReport summarises one venue-local business day (FR-A3).
//
// business_date is a date, not a timestamp range, because that is how
// orders record it: CreateOrder stamps the venue-local business day at
// placement time (see venue.BusinessDate), so the report is a plain
// equality filter and needs no timezone arithmetic of its own. The
// consequence is that a venue that changes timezone keeps its historical
// days intact rather than silently re-bucketing them.
//
// Cancelled orders are excluded from every figure — see the model's
// comment for why.
func (l *GetDayReportLogic) GetDayReport(in *v1_orderpb.GetDayReportRequest) (*v1_orderpb.DayReport, error) {
	if in.GetVenueId() == "" {
		return nil, apierr.Validation("venue_id is required")
	}

	settings, err := l.svcCtx.Venue.Get(l.ctx, in.GetVenueId())
	if err != nil {
		return nil, catalogError(err, "load venue settings")
	}

	// An omitted date means today, in the venue's own timezone — the
	// figure a manager checking mid-service actually wants.
	businessDate := settings.BusinessDate(time.Now())
	if raw := in.GetBusinessDate(); raw != "" {
		parsed, err := venue.ParseBusinessDate(raw)
		if err != nil {
			return nil, apierr.Validation("business_date must be YYYY-MM-DD")
		}
		businessDate = parsed
	}

	reports := model.NewReportModel(l.svcCtx.DB)

	totals, err := reports.DayTotals(l.ctx, in.GetVenueId(), businessDate)
	if err != nil {
		l.Errorf("day totals for venue %s on %s: %v", in.GetVenueId(), businessDate.Format(time.DateOnly), err)
		return nil, apierr.Internal("compute day totals")
	}

	topItems, err := reports.TopItems(l.ctx, in.GetVenueId(), businessDate, topItemsLimit)
	if err != nil {
		l.Errorf("top items for venue %s on %s: %v", in.GetVenueId(), businessDate.Format(time.DateOnly), err)
		return nil, apierr.Internal("compute top items")
	}

	currency := totals.Currency.String
	if currency == "" {
		// No orders that day, so no order carried a currency. Report the
		// venue's current one rather than an empty string, which a client
		// would have to special-case when formatting a zero total.
		currency = settings.Currency
	}

	out := &v1_orderpb.DayReport{
		VenueId:              in.GetVenueId(),
		BusinessDate:         businessDate.Format(time.DateOnly),
		OrdersCount:          totals.OrdersCount,
		RevenueMinor:         totals.RevenueMinor,
		Currency:             currency,
		AverageTicketMinor:   averageTicket(totals.RevenueMinor, totals.OrdersCount),
		AverageAcceptSeconds: totals.AvgAcceptSecs.Float64,
		AverageCookSeconds:   totals.AvgCookSecs.Float64,
		TopItems:             make([]*v1_orderpb.TopItem, 0, len(topItems)),
	}
	for _, it := range topItems {
		out.TopItems = append(out.TopItems, &v1_orderpb.TopItem{
			MenuItemId:   it.MenuItemID,
			Name:         it.Name,
			QtySold:      it.QtySold,
			RevenueMinor: it.RevenueMinor,
		})
	}
	return out, nil
}

// averageTicket rounds half up, for the same reason serviceCharge does:
// it is money on a report someone will reconcile against a till.
func averageTicket(revenueMinor int64, orders int32) int64 {
	if orders <= 0 {
		return 0
	}
	n := int64(orders)
	return (revenueMinor + n/2) / n
}
