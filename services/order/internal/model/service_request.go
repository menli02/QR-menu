package model

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

// Service request type values, mirroring service_requests.type.
const (
	RequestCallWaiter  = "call_waiter"
	RequestRequestBill = "request_bill"
)

// Service request status values, mirroring service_requests.status.
const (
	RequestOpen         = "open"
	RequestAcknowledged = "acknowledged"
	RequestResolved     = "resolved"
	RequestExpired      = "expired"
)

// ServiceRequest mirrors order_db.service_requests (FR-S1..S7).
type ServiceRequest struct {
	ID             string         `db:"id"`
	VenueID        string         `db:"venue_id"`
	TableID        string         `db:"table_id"`
	TableSessionID string         `db:"table_session_id"`
	Type           string         `db:"type"`
	Status         string         `db:"status"`
	Note           sql.NullString `db:"note"`
	CreatedAt      time.Time      `db:"created_at"`
	AcknowledgedAt sql.NullTime   `db:"acknowledged_at"`
	ResolvedAt     sql.NullTime   `db:"resolved_at"`
	ExpiresAt      time.Time      `db:"expires_at"`
}

type ServiceRequestModel struct {
	conn sqlx.Session
}

func NewServiceRequestModel(conn sqlx.Session) *ServiceRequestModel {
	return &ServiceRequestModel{conn: conn}
}

const serviceRequestCols = `id, venue_id, table_id, table_session_id, type, status, note,
	created_at, acknowledged_at, resolved_at, expires_at`

// CreateOrGetOpen is FR-S2's rate limit expressed as a constraint rather
// than a timer: service_requests_one_open_per_table_type_idx permits a
// single open row per (table, type), so a guest hammering "call waiter"
// gets the same request back instead of flooding the floor view.
//
// Returns created=false when an existing open request was returned. Like
// TableSessionModel.OpenOrJoin, the conflict target must repeat the
// partial index's WHERE clause for Postgres to use it as an arbiter.
func (m *ServiceRequestModel) CreateOrGetOpen(ctx context.Context, venueID, tableID, tableSessionID, reqType, note string, ttl time.Duration) (sr *ServiceRequest, created bool, err error) {
	var noteVal any
	if note != "" {
		noteVal = note
	}
	var r ServiceRequest
	err = m.conn.QueryRowCtx(ctx, &r, `
		INSERT INTO service_requests (venue_id, table_id, table_session_id, type, note, expires_at)
		VALUES ($1, $2, $3, $4, $5, now() + $6::interval)
		ON CONFLICT (table_id, type) WHERE status = 'open' DO NOTHING
		RETURNING `+serviceRequestCols,
		venueID, tableID, tableSessionID, reqType, noteVal, ttl.String())
	if err == nil {
		return &r, true, nil
	}
	if !errors.Is(err, sqlx.ErrNotFound) {
		return nil, false, err
	}
	existing, err := m.FindOpen(ctx, venueID, tableID, reqType)
	if err != nil {
		return nil, false, err
	}
	return existing, false, nil
}

func (m *ServiceRequestModel) FindOpen(ctx context.Context, venueID, tableID, reqType string) (*ServiceRequest, error) {
	var r ServiceRequest
	err := m.conn.QueryRowCtx(ctx, &r,
		`SELECT `+serviceRequestCols+` FROM service_requests
		 WHERE venue_id = $1 AND table_id = $2 AND type = $3 AND status = 'open'`,
		venueID, tableID, reqType)
	if err != nil {
		return nil, err
	}
	return &r, nil
}

func (m *ServiceRequestModel) FindByID(ctx context.Context, venueID, id string) (*ServiceRequest, error) {
	var r ServiceRequest
	err := m.conn.QueryRowCtx(ctx, &r,
		`SELECT `+serviceRequestCols+` FROM service_requests WHERE id = $1 AND venue_id = $2`, id, venueID)
	if err != nil {
		return nil, err
	}
	return &r, nil
}

// List is the floor view (FR-S3), defaulting to open requests, oldest
// first so the longest-waiting table is at the top.
func (m *ServiceRequestModel) List(ctx context.Context, venueID string, statuses []string) ([]ServiceRequest, error) {
	if len(statuses) == 0 {
		statuses = []string{RequestOpen}
	}
	var rows []ServiceRequest
	err := m.conn.QueryRowsCtx(ctx, &rows,
		`SELECT `+serviceRequestCols+` FROM service_requests
		 WHERE venue_id = $1 AND status = ANY($2::text[])
		 ORDER BY created_at, id`,
		venueID, pgTextArrayLiteral(statuses))
	if err != nil {
		return nil, err
	}
	return rows, nil
}

// UpdateStatus applies a transition with the same compare-and-set guard
// the order transitions use. acknowledged_at/resolved_at are stamped only
// on the transition that first reaches that state — COALESCE keeps an
// earlier acknowledgement's timestamp when the request later resolves.
func (m *ServiceRequestModel) UpdateStatus(ctx context.Context, venueID, id, to, from string) (*ServiceRequest, error) {
	var r ServiceRequest
	err := m.conn.QueryRowCtx(ctx, &r, `
		UPDATE service_requests SET
			status = $3,
			acknowledged_at = CASE WHEN $3 = 'acknowledged' THEN COALESCE(acknowledged_at, now()) ELSE acknowledged_at END,
			resolved_at     = CASE WHEN $3 = 'resolved'     THEN COALESCE(resolved_at, now())     ELSE resolved_at END
		WHERE id = $1 AND venue_id = $2 AND status = $4
		RETURNING `+serviceRequestCols,
		id, venueID, to, from)
	if err != nil {
		return nil, err
	}
	return &r, nil
}

// ResolveOpenForSession closes out anything still outstanding when the
// table session is paid and archived. A "bring me the bill" that is still
// open at the moment the bill is settled has, self-evidently, been dealt
// with; leaving it in the floor view until FR-S4's 15-minute expiry would
// send a waiter to a table that has already left.
//
// It returns the affected rows so the caller can emit one transition
// event per request, keeping the KDS and floor views in step.
func (m *ServiceRequestModel) ResolveOpenForSession(ctx context.Context, tableSessionID string) ([]ServiceRequest, error) {
	var rows []ServiceRequest
	err := m.conn.QueryRowsCtx(ctx, &rows, `
		UPDATE service_requests
		SET status = 'resolved', resolved_at = COALESCE(resolved_at, now())
		WHERE table_session_id = $1 AND status = 'open'
		RETURNING `+serviceRequestCols,
		tableSessionID)
	if err != nil {
		return nil, err
	}
	return rows, nil
}

// ExpireOverdue implements FR-S4. It is written as a set-based sweep so
// it can be driven either by a periodic job or opportunistically from a
// read path; either way it is idempotent and safe to run concurrently.
func (m *ServiceRequestModel) ExpireOverdue(ctx context.Context, venueID string) (int64, error) {
	res, err := m.conn.ExecCtx(ctx,
		`UPDATE service_requests SET status = 'expired'
		 WHERE venue_id = $1 AND status = 'open' AND expires_at <= now()`,
		venueID)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}
