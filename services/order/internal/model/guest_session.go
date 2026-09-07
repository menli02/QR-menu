package model

import (
	"context"
	"database/sql"
	"time"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

// GuestSession mirrors order_db.guest_sessions. Its id is not generated
// here: it is the guest_session_id identity already minted into the
// guest's JWT (see the migration's comment on the missing DEFAULT), so
// this row is a bookkeeping record of a session that already exists as a
// token, created lazily on the guest's first write.
type GuestSession struct {
	ID             string       `db:"id"`
	VenueID        string       `db:"venue_id"`
	TableID        string       `db:"table_id"`
	TableSessionID string       `db:"table_session_id"`
	IssuedAt       time.Time    `db:"issued_at"`
	ExpiresAt      time.Time    `db:"expires_at"`
	LastSeenAt     time.Time    `db:"last_seen_at"`
	RevokedAt      sql.NullTime `db:"revoked_at"`
}

type GuestSessionModel struct {
	conn sqlx.Session
}

func NewGuestSessionModel(conn sqlx.Session) *GuestSessionModel {
	return &GuestSessionModel{conn: conn}
}

const guestSessionCols = `id, venue_id, table_id, table_session_id, issued_at, expires_at,
	last_seen_at, revoked_at`

// EnsureExists registers the guest session against its table session on
// first write, or refreshes last_seen_at if it is already there (FR-O1's
// sliding activity window — the JWT `exp` claim remains the actual
// stateless expiry check; this column is bookkeeping for operators).
//
// ttl comes from the caller rather than being hardcoded so it always
// matches whatever identity minted into the token.
//
// The ON CONFLICT branch deliberately does not update table_session_id: a
// guest whose table session was closed under them must get a fresh token,
// not silently migrate onto the next party's bill.
func (m *GuestSessionModel) EnsureExists(ctx context.Context, id, venueID, tableID, tableSessionID string, ttl time.Duration) (*GuestSession, error) {
	var g GuestSession
	err := m.conn.QueryRowCtx(ctx, &g, `
		INSERT INTO guest_sessions (id, venue_id, table_id, table_session_id, expires_at)
		VALUES ($1, $2, $3, $4, now() + $5::interval)
		ON CONFLICT (id) DO UPDATE SET last_seen_at = now()
		RETURNING `+guestSessionCols,
		id, venueID, tableID, tableSessionID, ttl.String())
	if err != nil {
		return nil, err
	}
	return &g, nil
}

func (m *GuestSessionModel) FindByID(ctx context.Context, id string) (*GuestSession, error) {
	var g GuestSession
	err := m.conn.QueryRowCtx(ctx, &g,
		`SELECT `+guestSessionCols+` FROM guest_sessions WHERE id = $1`, id)
	if err != nil {
		return nil, err
	}
	return &g, nil
}

// RevokeForTableSession is called when a table session closes: the party
// has paid and left, so their tokens should stop being usable for new
// orders even though the JWTs themselves have not expired yet.
func (m *GuestSessionModel) RevokeForTableSession(ctx context.Context, tableSessionID string) error {
	_, err := m.conn.ExecCtx(ctx,
		`UPDATE guest_sessions SET revoked_at = now()
		 WHERE table_session_id = $1 AND revoked_at IS NULL`, tableSessionID)
	return err
}
