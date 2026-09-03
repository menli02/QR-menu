package model

import (
	"context"
	"database/sql"
	"time"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

// RefreshToken mirrors identity_db.refresh_tokens.
type RefreshToken struct {
	ID         string         `db:"id"`
	StaffID    string         `db:"staff_id"`
	TokenHash  string         `db:"token_hash"`
	IssuedAt   time.Time      `db:"issued_at"`
	ExpiresAt  time.Time      `db:"expires_at"`
	RevokedAt  sql.NullTime   `db:"revoked_at"`
	ReplacedBy sql.NullString `db:"replaced_by"`
}

// Active reports whether the token is neither expired nor revoked/replaced.
func (r RefreshToken) Active() bool {
	return !r.RevokedAt.Valid && time.Now().Before(r.ExpiresAt)
}

type RefreshTokenModel struct {
	conn sqlx.SqlConn
}

func NewRefreshTokenModel(conn sqlx.SqlConn) *RefreshTokenModel {
	return &RefreshTokenModel{conn: conn}
}

const refreshTokenCols = "id, staff_id, token_hash, issued_at, expires_at, revoked_at, replaced_by"

func (m *RefreshTokenModel) Insert(ctx context.Context, staffID, tokenHash string, expiresAt time.Time) (*RefreshToken, error) {
	var rt RefreshToken
	err := m.conn.QueryRowCtx(ctx, &rt, `
		INSERT INTO refresh_tokens (staff_id, token_hash, expires_at)
		VALUES ($1, $2, $3)
		RETURNING `+refreshTokenCols,
		staffID, tokenHash, expiresAt)
	if err != nil {
		return nil, err
	}
	return &rt, nil
}

func (m *RefreshTokenModel) FindByHash(ctx context.Context, tokenHash string) (*RefreshToken, error) {
	var rt RefreshToken
	err := m.conn.QueryRowCtx(ctx, &rt, `SELECT `+refreshTokenCols+` FROM refresh_tokens WHERE token_hash = $1`, tokenHash)
	if err != nil {
		return nil, err
	}
	return &rt, nil
}

// MarkReplaced revokes id and chains it to newID (rotation). A refresh
// found already revoked with a replaced_by set is how reuse of a
// rotated-away token is detected — see RevokeAllForStaff.
func (m *RefreshTokenModel) MarkReplaced(ctx context.Context, id, newID string) error {
	_, err := m.conn.ExecCtx(ctx, `
		UPDATE refresh_tokens SET revoked_at = now(), replaced_by = $2
		WHERE id = $1 AND revoked_at IS NULL`,
		id, newID)
	return err
}

// MarkRevoked is a plain revoke with no replacement (logout).
func (m *RefreshTokenModel) MarkRevoked(ctx context.Context, id string) error {
	_, err := m.conn.ExecCtx(ctx, `UPDATE refresh_tokens SET revoked_at = now() WHERE id = $1 AND revoked_at IS NULL`, id)
	return err
}

// RevokeAllForStaff revokes every still-active refresh token belonging to
// staffID. Called when Refresh sees a token that was already rotated away
// being replayed — the whole chain is treated as compromised.
func (m *RefreshTokenModel) RevokeAllForStaff(ctx context.Context, staffID string) error {
	_, err := m.conn.ExecCtx(ctx, `UPDATE refresh_tokens SET revoked_at = now() WHERE staff_id = $1 AND revoked_at IS NULL`, staffID)
	return err
}
