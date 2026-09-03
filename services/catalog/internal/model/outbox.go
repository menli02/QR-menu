package model

import (
	"context"
	"encoding/json"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

// OutboxModel inserts rows for the transactional outbox pattern
// (docs/TZ.md §8.3): the domain write and this insert happen in the same
// transaction (session is typically a tx Session, not a bare connection),
// so no event is ever published for a rolled-back write. There's no
// relay here yet — nothing publishes these rows to Kafka — matching
// config.go's KafkaBrokers field comment ("no producer constructed until
// the relay is implemented"). Rows sit ready for that follow-up.
type OutboxModel struct {
	conn sqlx.Session
}

func NewOutboxModel(conn sqlx.Session) *OutboxModel {
	return &OutboxModel{conn: conn}
}

// Insert writes one envelope row. payload is marshaled to JSON here so
// call sites pass a plain Go value, not pre-encoded bytes.
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
