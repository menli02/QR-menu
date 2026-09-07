package model

import (
	"context"
	"encoding/json"
	"time"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

// Kafka topics this service produces to (docs/TZ.md §8.3).
const (
	TopicOrder          = "qrmenu.order.v1"
	TopicServiceRequest = "qrmenu.service_request.v1"
)

// OutboxModel implements the producing half of the transactional outbox
// (docs/TZ.md §8.3): the domain write and the outbox insert share one
// Postgres transaction, so no event is published for a rolled-back write
// and no committed write loses its event if Kafka is down. The relay that
// drains these rows lives in internal/relay.
type OutboxModel struct {
	conn sqlx.Session
}

func NewOutboxModel(conn sqlx.Session) *OutboxModel {
	return &OutboxModel{conn: conn}
}

// Insert writes one envelope row. payload is marshaled here so call sites
// pass a plain Go value, not pre-encoded bytes.
func (m *OutboxModel) Insert(ctx context.Context, eventType, venueID, topic, partitionKey string, payload any, traceID string) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	_, err = m.conn.ExecCtx(ctx, `
		INSERT INTO outbox (event_type, venue_id, topic, partition_key, payload, trace_id)
		VALUES ($1, $2, $3, $4, $5, $6)`,
		eventType, venueID, topic, partitionKey, body, traceID)
	return err
}

// OutboxRow is one pending event as the relay sees it.
type OutboxRow struct {
	ID            string          `db:"id"`
	EventType     string          `db:"event_type"`
	SchemaVersion int             `db:"schema_version"`
	VenueID       string          `db:"venue_id"`
	Topic         string          `db:"topic"`
	PartitionKey  string          `db:"partition_key"`
	Payload       json.RawMessage `db:"payload"`
	TraceID       string          `db:"trace_id"`
	OccurredAt    time.Time       `db:"occurred_at"`
	Attempts      int             `db:"attempts"`
}

// ClaimPending locks a batch of unsent rows for one relay worker.
//
// FOR UPDATE SKIP LOCKED is what lets several relay replicas drain the
// same table without coordination: a row another worker already holds is
// skipped rather than waited on. Ordering by occurred_at preserves
// per-key ordering, which is the only ordering §8.3 guarantees.
//
// The caller must run this inside a transaction and hold it until the
// rows are published and marked — releasing the lock earlier would let a
// second worker republish them.
func (m *OutboxModel) ClaimPending(ctx context.Context, limit int) ([]OutboxRow, error) {
	var rows []OutboxRow
	err := m.conn.QueryRowsCtx(ctx, &rows, `
		SELECT id, event_type, schema_version, venue_id, topic, partition_key, payload,
		       COALESCE(trace_id, '') AS trace_id, occurred_at, attempts
		FROM outbox
		WHERE sent_at IS NULL
		ORDER BY occurred_at, id
		LIMIT $1
		FOR UPDATE SKIP LOCKED`,
		limit)
	if err != nil {
		return nil, err
	}
	return rows, nil
}

// MarkSent retires successfully published rows.
func (m *OutboxModel) MarkSent(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	_, err := m.conn.ExecCtx(ctx,
		`UPDATE outbox SET sent_at = now() WHERE id = ANY($1::uuid[])`, pgTextArrayLiteral(ids))
	return err
}

// MarkFailed records a publish failure so the row is retried on the next
// sweep and an operator can see why it is stuck. Rows are deliberately
// never dropped here: the DLQ decision belongs to the relay, which can
// see the attempt count.
func (m *OutboxModel) MarkFailed(ctx context.Context, ids []string, reason string) error {
	if len(ids) == 0 {
		return nil
	}
	_, err := m.conn.ExecCtx(ctx,
		`UPDATE outbox SET attempts = attempts + 1, last_error = $2 WHERE id = ANY($1::uuid[])`,
		pgTextArrayLiteral(ids), reason)
	return err
}
