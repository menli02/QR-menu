package model

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

// TableSession mirrors order_db.table_sessions. Currency is a snapshot of
// the venue's currency at open time (A1) so a mid-session venue currency
// change can't retroactively reinterpret an open bill.
type TableSession struct {
	ID              string         `db:"id"`
	VenueID         string         `db:"venue_id"`
	TableID         string         `db:"table_id"`
	Status          string         `db:"status"`
	OpenedAt        time.Time      `db:"opened_at"`
	ClosedAt        sql.NullTime   `db:"closed_at"`
	ClosedByStaffID sql.NullString `db:"closed_by_staff_id"`
	PaymentMethod   sql.NullString `db:"payment_method"`
	Currency        string         `db:"currency"`
	TotalMinor      int64          `db:"total_minor"`
	CreatedAt       time.Time      `db:"created_at"`
	UpdatedAt       time.Time      `db:"updated_at"`
}

const (
	TableSessionOpen   = "open"
	TableSessionClosed = "closed"
)

type TableSessionModel struct {
	conn sqlx.Session
}

func NewTableSessionModel(conn sqlx.Session) *TableSessionModel {
	return &TableSessionModel{conn: conn}
}

const tableSessionCols = `id, venue_id, table_id, status, opened_at, closed_at, closed_by_staff_id,
	payment_method, currency, total_minor, created_at, updated_at`

// OpenOrJoin is FR-O9: the first order at a free table opens a session,
// later orders from any guest at that table join it.
//
// The INSERT ... ON CONFLICT DO NOTHING + re-SELECT shape (rather than a
// SELECT-then-INSERT) is what makes this safe under two guests submitting
// simultaneously: table_sessions_one_open_per_table_idx is a *partial*
// unique index (WHERE status = 'open'), so the conflict target must name
// that same predicate for Postgres to use it as an arbiter.
//
// Callers must run this inside the same transaction as the order insert
// that motivated it — an open session with no order is a floor-view ghost.
func (m *TableSessionModel) OpenOrJoin(ctx context.Context, venueID, tableID, currency string) (*TableSession, error) {
	var s TableSession
	err := m.conn.QueryRowCtx(ctx, &s, `
		INSERT INTO table_sessions (venue_id, table_id, currency)
		VALUES ($1, $2, $3)
		ON CONFLICT (table_id) WHERE status = 'open' DO NOTHING
		RETURNING `+tableSessionCols,
		venueID, tableID, currency)
	if err == nil {
		return &s, nil
	}
	if !errors.Is(err, sqlx.ErrNotFound) {
		return nil, err
	}
	// DO NOTHING suppressed the insert, so RETURNING produced no row: a
	// session was already open. Join it.
	return m.FindOpenByTable(ctx, venueID, tableID)
}

// FindOpenByTable backs GetTableSession (FR-S5's per-table bill entry
// point and the guest's running total).
func (m *TableSessionModel) FindOpenByTable(ctx context.Context, venueID, tableID string) (*TableSession, error) {
	var s TableSession
	err := m.conn.QueryRowCtx(ctx, &s,
		`SELECT `+tableSessionCols+` FROM table_sessions
		 WHERE venue_id = $1 AND table_id = $2 AND status = 'open'`,
		venueID, tableID)
	if err != nil {
		return nil, err
	}
	return &s, nil
}

func (m *TableSessionModel) FindByID(ctx context.Context, venueID, id string) (*TableSession, error) {
	var s TableSession
	err := m.conn.QueryRowCtx(ctx, &s,
		`SELECT `+tableSessionCols+` FROM table_sessions WHERE id = $1 AND venue_id = $2`, id, venueID)
	if err != nil {
		return nil, err
	}
	return &s, nil
}

// ListActive is the floor view (FR-S3 context / §8.1 GET /floor/tables):
// every open session in the venue with its running total. Unpaginated by
// design — a venue has tens of tables, not thousands, and the floor view
// wants them all at once.
func (m *TableSessionModel) ListActive(ctx context.Context, venueID string) ([]TableSession, error) {
	var rows []TableSession
	err := m.conn.QueryRowsCtx(ctx, &rows,
		`SELECT `+tableSessionCols+` FROM table_sessions
		 WHERE venue_id = $1 AND status = 'open'
		 ORDER BY opened_at, id`,
		venueID)
	if err != nil {
		return nil, err
	}
	return rows, nil
}

// AddToTotal keeps the denormalized running total in step with an order
// write. delta may be negative (a cancellation). The schema's
// CHECK (total_minor >= 0) means a bug that over-subtracts fails loudly
// at commit rather than silently showing a negative bill.
func (m *TableSessionModel) AddToTotal(ctx context.Context, id string, deltaMinor int64) error {
	_, err := m.conn.ExecCtx(ctx,
		`UPDATE table_sessions SET total_minor = total_minor + $2 WHERE id = $1`, id, deltaMinor)
	return err
}

// Close marks the session paid and archived (FR-S6). It is a conditional
// UPDATE on status = 'open' rather than a read-then-write so that two
// staff closing the same table concurrently produce one close and one
// ErrNotFound, not a lost update.
func (m *TableSessionModel) Close(ctx context.Context, venueID, id, paymentMethod, staffID string) (*TableSession, error) {
	var s TableSession
	var staff any
	if staffID != "" {
		staff = staffID
	}
	err := m.conn.QueryRowCtx(ctx, &s, `
		UPDATE table_sessions
		SET status = 'closed', closed_at = now(), payment_method = $3, closed_by_staff_id = $4
		WHERE id = $1 AND venue_id = $2 AND status = 'open'
		RETURNING `+tableSessionCols,
		id, venueID, paymentMethod, staff)
	if err != nil {
		return nil, err
	}
	return &s, nil
}
