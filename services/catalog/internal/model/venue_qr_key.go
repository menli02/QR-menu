package model

import (
	"context"
	"database/sql"
	"time"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

// VenueQRKey mirrors catalog_db.venue_qr_keys — HMAC keys used to
// sign/verify table QR codes (FR-T2, FR-T4).
type VenueQRKey struct {
	VenueID    string       `db:"venue_id"`
	KeyVersion int32        `db:"key_version"`
	Secret     []byte       `db:"secret"`
	CreatedAt  time.Time    `db:"created_at"`
	ExpiresAt  sql.NullTime `db:"expires_at"` // NULL = current signing key
}

type VenueQRKeyModel struct {
	conn sqlx.Session
}

func NewVenueQRKeyModel(conn sqlx.Session) *VenueQRKeyModel {
	return &VenueQRKeyModel{conn: conn}
}

const venueQRKeyCols = "venue_id, key_version, secret, created_at, expires_at"

// FindCurrent returns the key a venue signs new QR codes with right now.
func (m *VenueQRKeyModel) FindCurrent(ctx context.Context, venueID string) (*VenueQRKey, error) {
	var k VenueQRKey
	err := m.conn.QueryRowCtx(ctx, &k,
		`SELECT `+venueQRKeyCols+` FROM venue_qr_keys WHERE venue_id = $1 AND expires_at IS NULL`, venueID)
	if err != nil {
		return nil, err
	}
	return &k, nil
}

// FindByVersion looks up a specific (possibly retired) key — ResolveTable
// verifies a presented signature against whichever key_version the QR
// link itself names, not necessarily the current one (FR-T4's rotation
// grace period).
func (m *VenueQRKeyModel) FindByVersion(ctx context.Context, venueID string, keyVersion int32) (*VenueQRKey, error) {
	var k VenueQRKey
	err := m.conn.QueryRowCtx(ctx, &k,
		`SELECT `+venueQRKeyCols+` FROM venue_qr_keys WHERE venue_id = $1 AND key_version = $2`, venueID, keyVersion)
	if err != nil {
		return nil, err
	}
	return &k, nil
}

// Insert adds a new current key. Callers rotating a key MUST call
// ExpireAt on the previous current key first, in the same transaction:
// venue_qr_keys_current_idx (partial unique on venue_id WHERE
// expires_at IS NULL) allows only one current key per venue, so
// inserting the new one before retiring the old one violates it —
// verified by a test that initially got this order backwards.
func (m *VenueQRKeyModel) Insert(ctx context.Context, venueID string, keyVersion int32, secret []byte) (*VenueQRKey, error) {
	var k VenueQRKey
	err := m.conn.QueryRowCtx(ctx, &k, `
		INSERT INTO venue_qr_keys (venue_id, key_version, secret)
		VALUES ($1, $2, $3)
		RETURNING `+venueQRKeyCols,
		venueID, keyVersion, secret)
	if err != nil {
		return nil, err
	}
	return &k, nil
}

// ExpireAt retires a key (typically the one just-superseded by Insert)
// by giving it an expiry — it stays valid for verification, per FR-T4's
// grace period, until that time passes.
func (m *VenueQRKeyModel) ExpireAt(ctx context.Context, venueID string, keyVersion int32, expiresAt time.Time) error {
	_, err := m.conn.ExecCtx(ctx,
		`UPDATE venue_qr_keys SET expires_at = $3 WHERE venue_id = $1 AND key_version = $2`,
		venueID, keyVersion, expiresAt)
	return err
}
