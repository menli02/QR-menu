package model

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

// pgUniqueViolation is Postgres's standard SQLSTATE for a unique-constraint
// violation (23505) — see https://www.postgresql.org/docs/current/errcodes-appendix.html.
const pgUniqueViolation = "23505"

// SigningKey mirrors identity_db.signing_keys. PublicJWK is the raw JSONB
// bytes of the public-key fields only — see that table's migration
// comment for why no private key material is stored here at all.
type SigningKey struct {
	Kid       string       `db:"kid"`
	Kty       string       `db:"kty"`
	Alg       string       `db:"alg"`
	PublicJWK []byte       `db:"public_jwk"`
	IsActive  bool         `db:"is_active"`
	CreatedAt time.Time    `db:"created_at"`
	RetiredAt sql.NullTime `db:"retired_at"`
}

type SigningKeyModel struct {
	conn sqlx.SqlConn
}

func NewSigningKeyModel(conn sqlx.SqlConn) *SigningKeyModel {
	return &SigningKeyModel{conn: conn}
}

// EnsureActive makes kid the active signing key, demoting whatever was
// previously active — unless kid is already registered (the common case:
// the same key file across a restart), which is a no-op. Deliberately
// idempotent and safe to call from any request path rather than once at
// startup: identity's DB connection is intentionally lazy (see
// service_context.go) so nothing here can turn a transient DB outage into
// a crash-looping boot.
//
// Two replicas racing this after a genuine key rotation can both decide
// "not registered yet" before either commits; the loser's INSERT then
// fails on the kid primary key (unique violation, SQLSTATE 23505), which
// is treated as success — the row it wanted to create already exists.
func (m *SigningKeyModel) EnsureActive(ctx context.Context, kid, kty, alg string, publicJWK []byte) error {
	err := m.conn.TransactCtx(ctx, func(ctx context.Context, session sqlx.Session) error {
		var exists bool
		if err := session.QueryRowCtx(ctx, &exists, `SELECT EXISTS(SELECT 1 FROM signing_keys WHERE kid = $1)`, kid); err != nil {
			return err
		}
		if exists {
			return nil
		}
		if _, err := session.ExecCtx(ctx, `UPDATE signing_keys SET is_active = false, retired_at = now() WHERE is_active`); err != nil {
			return err
		}
		_, err := session.ExecCtx(ctx, `
			INSERT INTO signing_keys (kid, kty, alg, public_jwk, is_active)
			VALUES ($1, $2, $3, $4, true)`,
			kid, kty, alg, publicJWK)
		return err
	})
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == pgUniqueViolation {
			return nil
		}
		return err
	}
	return nil
}

// List returns every signing key ever registered, oldest first. Retired
// keys must stay published until every token they signed has expired —
// there's no pruning job yet, matching the same category of known
// follow-up as the outbox/idempotency_keys purge jobs noted in
// migrations/order.
func (m *SigningKeyModel) List(ctx context.Context) ([]SigningKey, error) {
	var rows []SigningKey
	err := m.conn.QueryRowsCtx(ctx, &rows,
		`SELECT kid, kty, alg, public_jwk, is_active, created_at, retired_at FROM signing_keys ORDER BY created_at`)
	if err != nil {
		return nil, err
	}
	return rows, nil
}
