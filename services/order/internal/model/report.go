package model

import (
	"context"
	"database/sql"
	"time"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

// DayTotals is the scalar half of GetDayReport.
//
// Cancelled orders are excluded everywhere: a cancelled order is not
// revenue, and counting it would drag average_ticket_minor toward zero.
// The timing averages additionally skip rows whose stamps are NULL (an
// order that never reached that state, or one placed before migration
// 000003) — Postgres's avg() ignores NULLs, which is exactly the wanted
// behaviour, so this needs no filter of its own.
type DayTotals struct {
	OrdersCount   int32           `db:"orders_count"`
	RevenueMinor  int64           `db:"revenue_minor"`
	Currency      sql.NullString  `db:"currency"`
	AvgAcceptSecs sql.NullFloat64 `db:"avg_accept_seconds"`
	AvgCookSecs   sql.NullFloat64 `db:"avg_cook_seconds"`
}

// TopItem is one row of the report's best-sellers list.
type TopItem struct {
	MenuItemID   string `db:"menu_item_id"`
	Name         string `db:"name"`
	QtySold      int32  `db:"qty_sold"`
	RevenueMinor int64  `db:"revenue_minor"`
}

type ReportModel struct {
	conn sqlx.Session
}

func NewReportModel(conn sqlx.Session) *ReportModel {
	return &ReportModel{conn: conn}
}

// DayTotals aggregates one venue-local business day (FR-A3).
//
// min(currency) rather than a GROUP BY: every order in a venue shares the
// venue's currency (A1), so the aggregate is over one distinct value and
// min() is just the way to project it past the aggregation. On a day with
// no orders it is NULL, and the logic layer substitutes the venue's
// current currency.
func (m *ReportModel) DayTotals(ctx context.Context, venueID string, businessDate time.Time) (*DayTotals, error) {
	var t DayTotals
	err := m.conn.QueryRowCtx(ctx, &t, `
		SELECT
			count(*)::int                                      AS orders_count,
			COALESCE(sum(total_minor), 0)                      AS revenue_minor,
			min(currency)                                      AS currency,
			avg(extract(epoch FROM (accepted_at - placed_at))) AS avg_accept_seconds,
			avg(extract(epoch FROM (ready_at - accepted_at)))  AS avg_cook_seconds
		FROM orders
		WHERE venue_id = $1 AND business_date = $2 AND status <> 'cancelled'`,
		venueID, businessDate)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

// TopItems ranks the day's best sellers by revenue.
//
// Grouping is by menu_item_id *and* the snapshot name, so an item renamed
// mid-day appears once per name it was sold under rather than silently
// merging two different-looking lines. Cancelled orders and cancelled
// lines are both excluded.
func (m *ReportModel) TopItems(ctx context.Context, venueID string, businessDate time.Time, limit int) ([]TopItem, error) {
	var rows []TopItem
	err := m.conn.QueryRowsCtx(ctx, &rows, `
		SELECT
			oi.menu_item_id                     AS menu_item_id,
			oi.name                             AS name,
			COALESCE(sum(oi.qty), 0)::int       AS qty_sold,
			COALESCE(sum(oi.line_total_minor), 0) AS revenue_minor
		FROM order_items oi
		JOIN orders o ON o.id = oi.order_id
		WHERE o.venue_id = $1
		  AND o.business_date = $2
		  AND o.status <> 'cancelled'
		  AND oi.status <> 'cancelled'
		GROUP BY oi.menu_item_id, oi.name
		ORDER BY revenue_minor DESC, qty_sold DESC, oi.name
		LIMIT $3`,
		venueID, businessDate, limit)
	if err != nil {
		return nil, err
	}
	return rows, nil
}
