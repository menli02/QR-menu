package model

import (
	"context"
	"time"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

// ErrNotFound is returned by lookups that match no row. It is
// sqlx.ErrNotFound (== sql.ErrNoRows), re-exported so callers outside this
// package don't need to import sqlx just to compare errors.
var ErrNotFound = sqlx.ErrNotFound

// Staff mirrors identity_db.staff (migrations/identity/000002_schema.up.sql).
type Staff struct {
	ID           string    `db:"id"`
	VenueID      string    `db:"venue_id"`
	Name         string    `db:"name"`
	Email        string    `db:"email"`
	PasswordHash string    `db:"password_hash"`
	Role         string    `db:"role"`
	IsActive     bool      `db:"is_active"`
	CreatedAt    time.Time `db:"created_at"`
	UpdatedAt    time.Time `db:"updated_at"`
}

type StaffModel struct {
	conn sqlx.SqlConn
}

func NewStaffModel(conn sqlx.SqlConn) *StaffModel {
	return &StaffModel{conn: conn}
}

const staffCols = "id, venue_id, name, email, password_hash, role, is_active, created_at, updated_at"

// Insert creates a staff row. passwordHash must already be hashed (bcrypt)
// — this layer never sees a plaintext password.
func (m *StaffModel) Insert(ctx context.Context, venueID, name, email, passwordHash, role string) (*Staff, error) {
	var s Staff
	err := m.conn.QueryRowCtx(ctx, &s, `
		INSERT INTO staff (venue_id, name, email, password_hash, role)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING `+staffCols,
		venueID, name, email, passwordHash, role)
	if err != nil {
		return nil, err
	}
	return &s, nil
}

// FindByID is scoped to a venue, like every other staff lookup in this
// model. The scoping is the tenant boundary: an id from another venue
// simply does not match, so the caller gets ErrNotFound rather than
// another tenant's row — and rather than a "forbidden" that would confirm
// the id exists.
func (m *StaffModel) FindByID(ctx context.Context, venueID, id string) (*Staff, error) {
	var s Staff
	err := m.conn.QueryRowCtx(ctx, &s,
		`SELECT `+staffCols+` FROM staff WHERE id = $1 AND venue_id = $2`, id, venueID)
	if err != nil {
		return nil, err
	}
	return &s, nil
}

// FindByIDUnscoped looks a staff member up by id alone.
//
// Named unscoped so that using it is a visible decision. There is exactly
// one caller and one reason: Refresh follows the staff_id recorded on a
// refresh_tokens row it has already authenticated. That row *is* the proof
// of which staff member this is, so there is no venue to check against and
// nothing to cross — the token was issued to this person.
//
// Any other caller wants FindByID, which is scoped to a venue. Reaching
// for this one to avoid threading a venue_id through is how the
// cross-tenant read that FindByID now prevents gets reintroduced.
func (m *StaffModel) FindByIDUnscoped(ctx context.Context, id string) (*Staff, error) {
	var s Staff
	err := m.conn.QueryRowCtx(ctx, &s, `SELECT `+staffCols+` FROM staff WHERE id = $1`, id)
	if err != nil {
		return nil, err
	}
	return &s, nil
}

// FindByVenueAndEmail looks staff up the way Login does: email is CITEXT
// in the schema, so this comparison is already case-insensitive.
func (m *StaffModel) FindByVenueAndEmail(ctx context.Context, venueID, email string) (*Staff, error) {
	var s Staff
	err := m.conn.QueryRowCtx(ctx, &s, `SELECT `+staffCols+` FROM staff WHERE venue_id = $1 AND email = $2`, venueID, email)
	if err != nil {
		return nil, err
	}
	return &s, nil
}

// zeroUUID sorts before every real id column value (gen_random_uuid()
// never produces an all-zero UUID) and, unlike "", is a value Postgres can
// actually type as uuid for the tuple comparison below.
const zeroUUID = "00000000-0000-0000-0000-000000000000"

// List returns up to limit staff for venueID ordered by (created_at, id)
// strictly after cursor — keyset pagination (see pagination.go).
func (m *StaffModel) List(ctx context.Context, venueID string, after Cursor, limit int) ([]Staff, error) {
	afterID := after.ID
	if afterID == "" {
		afterID = zeroUUID
	}
	var rows []Staff
	err := m.conn.QueryRowsCtx(ctx, &rows, `
		SELECT `+staffCols+` FROM staff
		WHERE venue_id = $1 AND (created_at, id) > ($2, $3)
		ORDER BY created_at, id
		LIMIT $4`,
		venueID, after.CreatedAt, afterID, limit)
	if err != nil {
		return nil, err
	}
	return rows, nil
}

// Update changes the mutable profile fields. It is scoped to venueID as
// well as id: an UpdateStaffRequest whose Staff.venue_id doesn't match the
// row's actual venue affects zero rows (surfaced as ErrNotFound) rather
// than silently reassigning staff across venues.
func (m *StaffModel) Update(ctx context.Context, id, venueID, name, email, role string, isActive bool) (*Staff, error) {
	var s Staff
	err := m.conn.QueryRowCtx(ctx, &s, `
		UPDATE staff SET name = $3, email = $4, role = $5, is_active = $6
		WHERE id = $1 AND venue_id = $2
		RETURNING `+staffCols,
		id, venueID, name, email, role, isActive)
	if err != nil {
		return nil, err
	}
	return &s, nil
}

// SetPassword updates only the password hash. Returns false if no row
// matched (id) x (venueID).
func (m *StaffModel) SetPassword(ctx context.Context, id, venueID, passwordHash string) (bool, error) {
	res, err := m.conn.ExecCtx(ctx, `UPDATE staff SET password_hash = $3 WHERE id = $1 AND venue_id = $2`,
		id, venueID, passwordHash)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	return n > 0, err
}

// Deactivate soft-deletes staff (is_active = false) rather than issuing a
// hard DELETE. Other services hold staff_id as an opaque UUID with no FK
// (e.g. order_db.table_sessions.closed_by_staff_id) — a hard delete would
// silently orphan those references and make historical records unreadable
// ("closed by staff <gone>"). Login already rejects inactive staff, so
// this fully achieves "staff can no longer sign in."
func (m *StaffModel) Deactivate(ctx context.Context, id, venueID string) (bool, error) {
	res, err := m.conn.ExecCtx(ctx, `UPDATE staff SET is_active = false WHERE id = $1 AND venue_id = $2`,
		id, venueID)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	return n > 0, err
}
