package model

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"time"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

// Idempotency mirrors order_db.idempotency_keys (docs/TZ.md §7.4).
type Idempotency struct {
	VenueID      string          `db:"venue_id"`
	Endpoint     string          `db:"endpoint"`
	Key          string          `db:"key"`
	Fingerprint  string          `db:"fingerprint"`
	InProgress   bool            `db:"in_progress"`
	StatusCode   int32           `db:"status_code"`
	ResponseBody json.RawMessage `db:"response_body"`
	CreatedAt    time.Time       `db:"created_at"`
}

type IdempotencyModel struct {
	conn sqlx.Session
}

func NewIdempotencyModel(conn sqlx.Session) *IdempotencyModel {
	return &IdempotencyModel{conn: conn}
}

// Fingerprint is the SHA-256 of the canonical request body (§7.4). The
// caller decides what "canonical" means for its endpoint; this only
// hashes. Replaying a key with a different fingerprint is a client bug
// (or an attack) and is rejected rather than served a mismatched
// response.
func Fingerprint(parts ...string) string {
	h := sha256.New()
	for _, p := range parts {
		h.Write([]byte(p))
		h.Write([]byte{0}) // domain separator: ("a","bc") must not hash like ("ab","c")
	}
	return hex.EncodeToString(h.Sum(nil))
}

// Claim attempts to take ownership of (venueID, endpoint, key) for the
// current transaction.
//
// The whole handler runs inside one transaction: this INSERT, the domain
// writes, the outbox row and Complete all commit together. Two
// consequences worth stating, because they are what make the scheme
// correct rather than merely present:
//
//   - A committed row is always a *finished* request. A handler that
//     fails or crashes rolls its claim back with everything else, so the
//     same key is cleanly retryable — no poisoned key, no janitor needed.
//   - A concurrent second request with the same key blocks on this
//     INSERT (Postgres waits on the conflicting speculative insertion)
//     until lock_timeout fires, surfacing as IsLockUnavailable. That is
//     §7.4's REQUEST_IN_PROGRESS.
//
// owned=true means "proceed with the work". owned=false means prior is
// the completed record to replay.
func (m *IdempotencyModel) Claim(ctx context.Context, venueID, endpoint, key, fingerprint string) (prior *Idempotency, owned bool, err error) {
	var row Idempotency
	err = m.conn.QueryRowCtx(ctx, &row, `
		INSERT INTO idempotency_keys (venue_id, endpoint, key, fingerprint)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (venue_id, endpoint, key) DO NOTHING
		RETURNING venue_id, endpoint, key, fingerprint, in_progress,
		          COALESCE(status_code, 0) AS status_code,
		          COALESCE(response_body, 'null'::jsonb) AS response_body, created_at`,
		venueID, endpoint, key, fingerprint)
	if err == nil {
		return &row, true, nil
	}
	if !isNotFound(err) {
		return nil, false, err
	}

	existing, err := m.Find(ctx, venueID, endpoint, key)
	if err != nil {
		return nil, false, err
	}
	return existing, false, nil
}

// Find reads a key's record without claiming it. Used only for the
// read-only replay fast path; the authoritative check is Claim, under the
// row lock.
func (m *IdempotencyModel) Find(ctx context.Context, venueID, endpoint, key string) (*Idempotency, error) {
	var row Idempotency
	err := m.conn.QueryRowCtx(ctx, &row, `
		SELECT venue_id, endpoint, key, fingerprint, in_progress,
		       COALESCE(status_code, 0) AS status_code,
		       COALESCE(response_body, 'null'::jsonb) AS response_body, created_at
		FROM idempotency_keys
		WHERE venue_id = $1 AND endpoint = $2 AND key = $3`,
		venueID, endpoint, key)
	if err != nil {
		return nil, err
	}
	return &row, nil
}

// Complete stores the response that a replay of this key should return.
// Called at the end of the same transaction that claimed the key.
func (m *IdempotencyModel) Complete(ctx context.Context, venueID, endpoint, key string, statusCode int32, body []byte) error {
	_, err := m.conn.ExecCtx(ctx, `
		UPDATE idempotency_keys
		SET in_progress = false, status_code = $4, response_body = $5
		WHERE venue_id = $1 AND endpoint = $2 AND key = $3`,
		venueID, endpoint, key, statusCode, body)
	return err
}

// PurgeOlderThan is the daily cleanup job's query (§7.4: 24h TTL).
func (m *IdempotencyModel) PurgeOlderThan(ctx context.Context, age time.Duration) (int64, error) {
	res, err := m.conn.ExecCtx(ctx,
		`DELETE FROM idempotency_keys WHERE created_at < now() - $1::interval`, age.String())
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

func isNotFound(err error) bool {
	return errors.Is(err, sqlx.ErrNotFound)
}
