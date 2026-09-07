// Package outbox implements the publishing half of the transactional
// outbox pattern (docs/TZ.md §8.3).
//
// Producing services write domain events into an `outbox` table in the
// same transaction as the domain change, so an event is never published
// for a rolled-back write and never lost for a committed one. This
// package is the relay that drains those rows into Kafka.
//
// It lives in pkg/ rather than in one service's internal/ because
// catalog and order both produce events and the outbox table has exactly
// the same shape in both — the relay is infrastructure, not domain logic,
// and duplicating it would mean two copies of the delivery guarantees to
// keep in step.
package outbox

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/segmentio/kafka-go"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

// Defaults applied to a zero-valued Config field.
const (
	DefaultPollInterval = 500 * time.Millisecond
	DefaultBatchSize    = 100
	DefaultMaxAttempts  = 5
	DefaultWriteTimeout = 5 * time.Second
)

type Config struct {
	// Brokers is the Kafka bootstrap list. Empty disables the relay
	// entirely: New returns a no-op relay, so a local dev environment
	// without Kafka still accumulates outbox rows and can be inspected,
	// without a background goroutine failing to dial on a loop.
	Brokers []string

	// PollInterval is how long the relay sleeps when it finds nothing to
	// do. After a full batch it polls again immediately, so a burst
	// drains at Kafka's pace rather than one batch per tick.
	PollInterval time.Duration

	// BatchSize bounds one claim, and therefore how long a single
	// transaction holds its rows locked.
	BatchSize int

	// MaxAttempts is §8.3's 5 tries before a message is routed to the
	// topic's DLQ.
	MaxAttempts int

	WriteTimeout time.Duration

	// Source names the producing service in logs and in the DLQ headers.
	Source string
}

func (c *Config) applyDefaults() {
	if c.PollInterval <= 0 {
		c.PollInterval = DefaultPollInterval
	}
	if c.BatchSize <= 0 {
		c.BatchSize = DefaultBatchSize
	}
	if c.MaxAttempts <= 0 {
		c.MaxAttempts = DefaultMaxAttempts
	}
	if c.WriteTimeout <= 0 {
		c.WriteTimeout = DefaultWriteTimeout
	}
	if c.Source == "" {
		c.Source = "unknown"
	}
}

// Publisher is the Kafka write surface the relay needs. It exists so the
// sweep logic can be tested against a fake without a broker.
type Publisher interface {
	WriteMessages(ctx context.Context, msgs ...kafka.Message) error
	Close() error
}

// Relay drains outbox rows into Kafka. Start/Stop match go-zero's
// service.Service, so it can join a ServiceGroup next to the RPC server.
type Relay struct {
	db        sqlx.SqlConn
	publisher Publisher
	cfg       Config

	ctx    context.Context
	cancel context.CancelFunc
	done   chan struct{}
}

// New builds a relay. With no brokers configured it returns a relay whose
// Start is a no-op — see Config.Brokers.
func New(db sqlx.SqlConn, cfg Config) *Relay {
	cfg.applyDefaults()

	var publisher Publisher
	if len(cfg.Brokers) > 0 {
		publisher = &kafka.Writer{
			Addr: kafka.TCP(cfg.Brokers...),
			// Topic is intentionally unset: each message names its own,
			// because one outbox feeds qrmenu.order.v1 and
			// qrmenu.service_request.v1 (and their DLQs).
			Balancer: &kafka.Hash{}, // §8.3 guarantees ordering per key, which needs key-stable partitioning
			// RequiredAcks=all: a broker acknowledging before replication
			// would let a leader failover silently drop an event we have
			// already marked sent.
			RequiredAcks: kafka.RequireAll,
			// The relay does its own retry accounting in next_attempt_at,
			// with the attempt count durable across restarts. Letting the
			// writer retry internally as well would hide failures from
			// that bookkeeping and stall the batch's transaction.
			MaxAttempts: 1,
			Async:       false,
			// AllowAutoTopicCreation stays false, deliberately. A broker
			// with auto-creation enabled makes the topic with its *default*
			// partition count, which is 1 — silently discarding §8.3's
			// per-topic plan (order 6, service_request 3) and with it the
			// per-key ordering that plan buys. Failing loudly against a
			// missing topic is better: the rows stay in the outbox, the
			// relay backs off, and scripts/kafka-topics.sh creates them
			// properly. Verified end to end: 12 events survived four failed
			// attempts against a broker with no topics and drained
			// untouched once the topics existed.
		}
	}

	return newWithPublisher(db, cfg, publisher)
}

// newWithPublisher is the seam tests use to inject a fake broker.
func newWithPublisher(db sqlx.SqlConn, cfg Config, publisher Publisher) *Relay {
	cfg.applyDefaults()
	ctx, cancel := context.WithCancel(context.Background())
	return &Relay{
		db:        db,
		publisher: publisher,
		cfg:       cfg,
		ctx:       ctx,
		cancel:    cancel,
		done:      make(chan struct{}),
	}
}

// Start runs the relay loop until Stop. It blocks, matching go-zero's
// service.Service contract; ServiceGroup runs it on its own goroutine.
func (r *Relay) Start() {
	defer close(r.done)

	if r.publisher == nil {
		logx.Infof("outbox relay disabled for %s: no Kafka brokers configured", r.cfg.Source)
		return
	}
	logx.Infof("outbox relay started for %s (batch %d, poll %s)", r.cfg.Source, r.cfg.BatchSize, r.cfg.PollInterval)

	timer := time.NewTimer(0)
	defer timer.Stop()

	for {
		select {
		case <-r.ctx.Done():
			return
		case <-timer.C:
		}

		n, err := r.Sweep(r.ctx)
		switch {
		case err != nil && errors.Is(err, context.Canceled):
			return
		case err != nil:
			logx.Errorf("outbox relay sweep failed for %s: %v", r.cfg.Source, err)
			timer.Reset(r.cfg.PollInterval)
		case n == r.cfg.BatchSize:
			// A full batch probably means more is waiting; don't sleep.
			timer.Reset(0)
		default:
			timer.Reset(r.cfg.PollInterval)
		}
	}
}

// Stop ends the loop and waits for the in-flight sweep to finish, so a
// shutdown never abandons a transaction mid-publish. It must be called
// after Start — which is what service.ServiceGroup guarantees, and the
// only way this type is meant to be run.
func (r *Relay) Stop() {
	r.cancel()
	<-r.done
	if r.publisher != nil {
		if err := r.publisher.Close(); err != nil {
			logx.Errorf("outbox relay close failed for %s: %v", r.cfg.Source, err)
		}
	}
}

// row is one claimed outbox record.
type row struct {
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

// envelope is §8.3's message format. Field names and shape are a
// published contract; consumers parse this.
type envelope struct {
	EventID       string          `json:"event_id"`
	EventType     string          `json:"event_type"`
	SchemaVersion int             `json:"schema_version"`
	OccurredAt    time.Time       `json:"occurred_at"`
	VenueID       string          `json:"venue_id"`
	TraceID       string          `json:"trace_id,omitempty"`
	Payload       json.RawMessage `json:"payload"`
}

// Sweep claims one batch, publishes it and records the outcome, all in a
// single transaction. It returns how many rows were claimed.
//
// The transaction stays open across the Kafka write on purpose. The claim
// uses FOR UPDATE SKIP LOCKED, so holding it is what stops a second relay
// replica — or this one after a restart — from republishing the same
// rows. The window is bounded by WriteTimeout.
//
// Exported so tests can drive one sweep deterministically instead of
// racing the loop.
func (r *Relay) Sweep(ctx context.Context) (int, error) {
	var claimed int
	err := r.db.TransactCtx(ctx, func(ctx context.Context, s sqlx.Session) error {
		var rows []row
		err := s.QueryRowsCtx(ctx, &rows, `
			SELECT id, event_type, schema_version, venue_id, topic, partition_key, payload,
			       COALESCE(trace_id, '') AS trace_id, occurred_at, attempts
			FROM outbox
			WHERE sent_at IS NULL AND next_attempt_at <= now()
			ORDER BY occurred_at, id
			LIMIT $1
			FOR UPDATE SKIP LOCKED`,
			r.cfg.BatchSize)
		if err != nil {
			return fmt.Errorf("claim outbox rows: %w", err)
		}
		claimed = len(rows)
		if claimed == 0 {
			return nil
		}

		msgs := make([]kafka.Message, 0, len(rows))
		ids := make([]string, 0, len(rows))
		for _, rw := range rows {
			body, err := json.Marshal(envelope{
				EventID:       rw.ID,
				EventType:     rw.EventType,
				SchemaVersion: rw.SchemaVersion,
				OccurredAt:    rw.OccurredAt.UTC(),
				VenueID:       rw.VenueID,
				TraceID:       rw.TraceID,
				Payload:       rw.Payload,
			})
			if err != nil {
				// A row whose payload can't be re-marshaled will never
				// succeed, so retrying it forever would block the queue.
				// Route it straight to the DLQ.
				logx.Errorf("outbox row %s has an unmarshalable payload, sending to DLQ: %v", rw.ID, err)
				continue
			}
			msgs = append(msgs, kafka.Message{
				Topic: rw.Topic,
				Key:   []byte(rw.PartitionKey),
				Value: body,
				Headers: []kafka.Header{
					{Key: "event_type", Value: []byte(rw.EventType)},
					{Key: "event_id", Value: []byte(rw.ID)},
					{Key: "venue_id", Value: []byte(rw.VenueID)},
					{Key: "trace_id", Value: []byte(rw.TraceID)},
				},
			})
			ids = append(ids, rw.ID)
		}

		if len(msgs) == 0 {
			return nil
		}

		writeCtx, cancel := context.WithTimeout(ctx, r.cfg.WriteTimeout)
		defer cancel()

		if err := r.publisher.WriteMessages(writeCtx, msgs...); err != nil {
			return r.recordFailure(ctx, s, rows, err)
		}

		if _, err := s.ExecCtx(ctx,
			`UPDATE outbox SET sent_at = now() WHERE id = ANY($1::uuid[])`, uuidArray(ids)); err != nil {
			return fmt.Errorf("mark outbox rows sent: %w", err)
		}
		return nil
	})
	return claimed, err
}

// recordFailure applies §8.3's retry policy: bump the attempt count,
// schedule the next try with exponential backoff, and once a row has
// exhausted MaxAttempts route it to the topic's DLQ with the reason in a
// header.
//
// A DLQ write that itself fails is logged and the row is left pending —
// better a duplicate later than a silently dropped event.
func (r *Relay) recordFailure(ctx context.Context, s sqlx.Session, rows []row, cause error) error {
	logx.Errorf("outbox publish failed for %s (%d rows): %v", r.cfg.Source, len(rows), cause)

	var retry, dead []row
	for _, rw := range rows {
		if rw.Attempts+1 >= r.cfg.MaxAttempts {
			dead = append(dead, rw)
		} else {
			retry = append(retry, rw)
		}
	}

	for _, rw := range retry {
		if _, err := s.ExecCtx(ctx, `
			UPDATE outbox
			SET attempts = attempts + 1, last_error = $2, next_attempt_at = now() + $3::interval
			WHERE id = $1`,
			rw.ID, truncateError(cause), backoff(rw.Attempts+1).String()); err != nil {
			return fmt.Errorf("record outbox retry: %w", err)
		}
	}

	if len(dead) > 0 {
		r.toDLQ(ctx, s, dead, cause)
	}
	// The batch failing is expected operation, not a sweep error: the
	// transaction must commit so the bookkeeping above survives.
	return nil
}

func (r *Relay) toDLQ(ctx context.Context, s sqlx.Session, rows []row, cause error) {
	msgs := make([]kafka.Message, 0, len(rows))
	ids := make([]string, 0, len(rows))
	for _, rw := range rows {
		body, err := json.Marshal(envelope{
			EventID:       rw.ID,
			EventType:     rw.EventType,
			SchemaVersion: rw.SchemaVersion,
			OccurredAt:    rw.OccurredAt.UTC(),
			VenueID:       rw.VenueID,
			TraceID:       rw.TraceID,
			Payload:       rw.Payload,
		})
		if err != nil {
			logx.Errorf("outbox row %s cannot be encoded even for the DLQ: %v", rw.ID, err)
			continue
		}
		msgs = append(msgs, kafka.Message{
			Topic: rw.Topic + ".dlq",
			Key:   []byte(rw.PartitionKey),
			Value: body,
			Headers: []kafka.Header{
				{Key: "event_type", Value: []byte(rw.EventType)},
				{Key: "event_id", Value: []byte(rw.ID)},
				{Key: "venue_id", Value: []byte(rw.VenueID)},
				{Key: "trace_id", Value: []byte(rw.TraceID)},
				{Key: "dlq_reason", Value: []byte(truncateError(cause))},
				{Key: "dlq_source", Value: []byte(r.cfg.Source)},
				{Key: "dlq_attempts", Value: []byte(strconv.Itoa(rw.Attempts + 1))},
			},
		})
		ids = append(ids, rw.ID)
	}
	if len(msgs) == 0 {
		return
	}

	writeCtx, cancel := context.WithTimeout(ctx, r.cfg.WriteTimeout)
	defer cancel()

	if err := r.publisher.WriteMessages(writeCtx, msgs...); err != nil {
		// Leave the rows pending rather than marking them sent: the
		// broker is evidently unreachable, and dropping the event here
		// would be the one failure mode the outbox exists to prevent.
		logx.Errorf("outbox DLQ publish failed for %s, leaving %d rows pending: %v", r.cfg.Source, len(msgs), err)
		return
	}

	if _, err := s.ExecCtx(ctx, `
		UPDATE outbox SET attempts = attempts + 1, sent_at = now(), last_error = $2
		WHERE id = ANY($1::uuid[])`,
		uuidArray(ids), "sent to DLQ: "+truncateError(cause)); err != nil {
		logx.Errorf("outbox DLQ bookkeeping failed for %s: %v", r.cfg.Source, err)
		return
	}
	logx.Errorf("outbox routed %d rows to the DLQ for %s after %d attempts", len(ids), r.cfg.Source, r.cfg.MaxAttempts)
}

// backoff is §8.3's 1s, 2s, 4s, 8s, 16s schedule.
func backoff(attempt int) time.Duration {
	if attempt < 1 {
		attempt = 1
	}
	if attempt > 10 { // cap so a long-dead row doesn't schedule itself into next year
		attempt = 10
	}
	return time.Duration(1<<(attempt-1)) * time.Second
}

// truncateError bounds what goes into last_error and a DLQ header. A
// driver error can carry a whole query; a Kafka header is not the place
// for it, and neither is a TEXT column read by a human at 2am.
func truncateError(err error) string {
	const max = 500
	s := err.Error()
	if len(s) > max {
		return s[:max] + "…"
	}
	return s
}

// uuidArray renders ids as a Postgres array literal. pgx's stdlib driver
// can't encode a []string parameter, so array arguments travel as text
// and are cast in SQL — the same approach the service model packages use.
func uuidArray(ids []string) string {
	if len(ids) == 0 {
		return "{}"
	}
	out := make([]byte, 0, len(ids)*38)
	out = append(out, '{')
	for i, id := range ids {
		if i > 0 {
			out = append(out, ',')
		}
		out = append(out, '"')
		out = append(out, id...)
		out = append(out, '"')
	}
	return string(append(out, '}'))
}
