package outbox

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/segmentio/kafka-go"
	"github.com/zeromicro/go-zero/core/stores/postgres"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

// ---------------------------------------------------------------------
// Pure helpers
// ---------------------------------------------------------------------

func TestBackoff(t *testing.T) {
	// docs/TZ.md §8.3: 1s, 2s, 4s, 8s, 16s.
	want := []time.Duration{time.Second, 2 * time.Second, 4 * time.Second, 8 * time.Second, 16 * time.Second}
	for i, w := range want {
		if got := backoff(i + 1); got != w {
			t.Errorf("backoff(%d) = %s, want %s", i+1, got, w)
		}
	}
	if got := backoff(0); got != time.Second {
		t.Errorf("backoff(0) = %s, want 1s", got)
	}
	// Capped so a long-dead row can't schedule itself years out.
	if got := backoff(100); got != 512*time.Second {
		t.Errorf("backoff(100) = %s, want 512s", got)
	}
}

func TestUUIDArray(t *testing.T) {
	if got := uuidArray(nil); got != "{}" {
		t.Errorf("uuidArray(nil) = %q, want {}", got)
	}
	if got := uuidArray([]string{"a"}); got != `{"a"}` {
		t.Errorf(`uuidArray(["a"]) = %q, want {"a"}`, got)
	}
	if got := uuidArray([]string{"a", "b"}); got != `{"a","b"}` {
		t.Errorf(`uuidArray(["a","b"]) = %q, want {"a","b"}`, got)
	}
}

func TestTruncateError(t *testing.T) {
	short := errors.New("boom")
	if got := truncateError(short); got != "boom" {
		t.Errorf("truncateError(short) = %q", got)
	}
	long := errors.New(strings.Repeat("x", 900))
	got := truncateError(long)
	if len([]rune(got)) != 501 { // 500 chars + the ellipsis
		t.Errorf("truncateError(long) produced %d runes, want 501", len([]rune(got)))
	}
	if !strings.HasSuffix(got, "…") {
		t.Error("a truncated error should be marked with an ellipsis")
	}
}

func TestConfigDefaults(t *testing.T) {
	var c Config
	c.applyDefaults()
	if c.PollInterval != DefaultPollInterval || c.BatchSize != DefaultBatchSize ||
		c.MaxAttempts != DefaultMaxAttempts || c.WriteTimeout != DefaultWriteTimeout {
		t.Errorf("zero config did not pick up defaults: %+v", c)
	}
	if c.Source != "unknown" {
		t.Errorf("Source = %q, want unknown", c.Source)
	}

	explicit := Config{PollInterval: time.Second, BatchSize: 7, MaxAttempts: 2, WriteTimeout: time.Minute, Source: "svc"}
	explicit.applyDefaults()
	if explicit.BatchSize != 7 || explicit.MaxAttempts != 2 || explicit.Source != "svc" {
		t.Errorf("explicit config was overwritten: %+v", explicit)
	}
}

// TestRelayWithoutBrokersIsANoOp covers the local-dev path: no Kafka
// configured must not mean a goroutine failing to dial on a loop.
func TestRelayWithoutBrokersIsANoOp(t *testing.T) {
	r := New(nil, Config{Source: "test"})
	if r.publisher != nil {
		t.Fatal("a relay with no brokers should have no publisher")
	}

	done := make(chan struct{})
	go func() { r.Start(); close(done) }()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Start did not return immediately with no brokers configured")
	}
	r.Stop() // must not panic or block
}

// ---------------------------------------------------------------------
// Sweep, against real Postgres and a fake broker
// ---------------------------------------------------------------------

// fakePublisher records what the relay tried to send and can be made to
// fail on demand.
type fakePublisher struct {
	mu    sync.Mutex
	sent  []kafka.Message
	err   error
	calls int
}

func (p *fakePublisher) WriteMessages(_ context.Context, msgs ...kafka.Message) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.calls++
	if p.err != nil {
		return p.err
	}
	p.sent = append(p.sent, msgs...)
	return nil
}

func (p *fakePublisher) Close() error { return nil }

func (p *fakePublisher) messages() []kafka.Message {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]kafka.Message(nil), p.sent...)
}

// testConn connects to the local-dev order_db (`make infra-up migrate-up`)
// and skips if it isn't reachable. The sweep's correctness lives entirely
// in SQL — FOR UPDATE SKIP LOCKED, the partial index, the transaction
// boundary — so a mock database would test nothing worth testing.
func testConn(t *testing.T) sqlx.SqlConn {
	t.Helper()
	conn := postgres.New("postgres://qrmenu:qrmenu@127.0.0.1:5437/order_db?sslmode=disable")
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	var one int
	if err := conn.QueryRowCtx(ctx, &one, "SELECT 1"); err != nil {
		t.Skipf("order_db not reachable (%v) — run `make infra-up migrate-up` to enable these integration tests", err)
	}
	return conn
}

// seedRow inserts one pending outbox row and registers its cleanup.
//
// dueIn is expressed relative to Postgres's own clock, not Go's. That is
// not incidental: the relay's claim filters on `next_attempt_at <= now()`
// evaluated by the server, and an earlier version of this helper wrote an
// absolute time.Now() from the test process. Postgres in Docker runs a
// fraction of a millisecond behind the host, so a row seeded as "due now"
// was intermittently in the server's future and silently skipped —
// producing flaky failures in whichever test happened to run fastest.
// Letting the database compute both sides of the comparison removes the
// skew entirely.
func seedRow(t *testing.T, conn sqlx.SqlConn, topic, eventType string, attempts int, dueIn time.Duration) string {
	t.Helper()
	id := uuid.NewString()
	venueID := uuid.NewString()
	_, err := conn.ExecCtx(context.Background(), `
		INSERT INTO outbox (id, event_type, venue_id, topic, partition_key, payload, trace_id, attempts, next_attempt_at)
		VALUES ($1, $2, $3, $4, $5, $6::jsonb, $7, $8, now() + $9::interval)`,
		id, eventType, venueID, topic, "key-"+id, `{"hello":"world"}`, "trace-"+id, attempts, dueIn.String())
	if err != nil {
		t.Fatalf("seed outbox row: %v", err)
	}
	t.Cleanup(func() { _, _ = conn.ExecCtx(context.Background(), "DELETE FROM outbox WHERE id = $1", id) })
	return id
}

// dueNow and dueLater are the two states seedRow is ever asked for. A
// small negative offset for "now" keeps the row unambiguously in the
// past for any statement that follows.
const (
	dueNow   = -time.Second
	dueLater = time.Hour
)

type rowState struct {
	SentAt        *time.Time `db:"sent_at"`
	Attempts      int        `db:"attempts"`
	LastError     *string    `db:"last_error"`
	NextAttemptAt time.Time  `db:"next_attempt_at"`
}

func readRow(t *testing.T, conn sqlx.SqlConn, id string) rowState {
	t.Helper()
	var st rowState
	err := conn.QueryRowCtx(context.Background(), &st,
		`SELECT sent_at, attempts, last_error, next_attempt_at FROM outbox WHERE id = $1`, id)
	if err != nil {
		t.Fatalf("read outbox row %s: %v", id, err)
	}
	return st
}

func TestSweepPublishesAndMarksSent(t *testing.T) {
	conn := testConn(t)
	id := seedRow(t, conn, "qrmenu.order.v1", "order.placed", 0, dueNow)

	pub := &fakePublisher{}
	r := newWithPublisher(conn, Config{Source: "order.rpc", BatchSize: 10}, pub)

	n, err := r.Sweep(context.Background())
	if err != nil {
		t.Fatalf("Sweep: %v", err)
	}
	if n == 0 {
		t.Fatal("Sweep claimed no rows")
	}

	var found *kafka.Message
	for _, m := range pub.messages() {
		for _, h := range m.Headers {
			if h.Key == "event_id" && string(h.Value) == id {
				msg := m
				found = &msg
			}
		}
	}
	if found == nil {
		t.Fatalf("row %s was not published", id)
	}

	if found.Topic != "qrmenu.order.v1" {
		t.Errorf("topic = %q, want qrmenu.order.v1", found.Topic)
	}
	if string(found.Key) != "key-"+id {
		t.Errorf("key = %q, want key-%s", found.Key, id)
	}

	// docs/TZ.md §8.3's envelope.
	var env struct {
		EventID       string          `json:"event_id"`
		EventType     string          `json:"event_type"`
		SchemaVersion int             `json:"schema_version"`
		OccurredAt    time.Time       `json:"occurred_at"`
		VenueID       string          `json:"venue_id"`
		TraceID       string          `json:"trace_id"`
		Payload       json.RawMessage `json:"payload"`
	}
	if err := json.Unmarshal(found.Value, &env); err != nil {
		t.Fatalf("envelope is not valid JSON: %v", err)
	}
	if env.EventID != id {
		t.Errorf("event_id = %q, want %q", env.EventID, id)
	}
	if env.EventType != "order.placed" {
		t.Errorf("event_type = %q, want order.placed", env.EventType)
	}
	if env.SchemaVersion != 1 {
		t.Errorf("schema_version = %d, want 1", env.SchemaVersion)
	}
	if env.OccurredAt.IsZero() {
		t.Error("occurred_at is missing")
	}
	if string(env.Payload) != `{"hello": "world"}` && string(env.Payload) != `{"hello":"world"}` {
		t.Errorf("payload = %s, want the row's payload verbatim", env.Payload)
	}

	st := readRow(t, conn, id)
	if st.SentAt == nil {
		t.Error("a published row should have sent_at set")
	}

	// A second sweep must not republish it.
	pub.sent = nil
	if _, err := r.Sweep(context.Background()); err != nil {
		t.Fatalf("second Sweep: %v", err)
	}
	for _, m := range pub.messages() {
		for _, h := range m.Headers {
			if h.Key == "event_id" && string(h.Value) == id {
				t.Fatal("a sent row was published twice")
			}
		}
	}
}

func TestSweepSkipsRowsNotYetDue(t *testing.T) {
	conn := testConn(t)
	id := seedRow(t, conn, "qrmenu.order.v1", "order.placed", 1, dueLater)

	pub := &fakePublisher{}
	r := newWithPublisher(conn, Config{Source: "order.rpc", BatchSize: 50}, pub)
	if _, err := r.Sweep(context.Background()); err != nil {
		t.Fatalf("Sweep: %v", err)
	}

	for _, m := range pub.messages() {
		for _, h := range m.Headers {
			if h.Key == "event_id" && string(h.Value) == id {
				t.Fatal("a row scheduled for the future was published early")
			}
		}
	}
	if st := readRow(t, conn, id); st.SentAt != nil {
		t.Error("a backed-off row should stay pending")
	}
}

func TestSweepSchedulesRetryOnFailure(t *testing.T) {
	conn := testConn(t)
	id := seedRow(t, conn, "qrmenu.order.v1", "order.placed", 0, dueNow)

	pub := &fakePublisher{err: errors.New("broker down")}
	r := newWithPublisher(conn, Config{Source: "order.rpc", BatchSize: 50, MaxAttempts: 5}, pub)

	// A publish failure is normal operation, not a sweep error: the
	// bookkeeping has to commit.
	if _, err := r.Sweep(context.Background()); err != nil {
		t.Fatalf("Sweep returned an error for a publish failure: %v", err)
	}

	st := readRow(t, conn, id)
	if st.SentAt != nil {
		t.Error("a failed row must not be marked sent")
	}
	if st.Attempts != 1 {
		t.Errorf("attempts = %d, want 1", st.Attempts)
	}
	if st.LastError == nil || !strings.Contains(*st.LastError, "broker down") {
		t.Errorf("last_error = %v, want it to mention the cause", st.LastError)
	}
	if !st.NextAttemptAt.After(time.Now().Add(500 * time.Millisecond)) {
		t.Errorf("next_attempt_at = %s, want roughly 1s in the future", st.NextAttemptAt)
	}
}

func TestSweepRoutesToDLQAfterMaxAttempts(t *testing.T) {
	conn := testConn(t)
	// One attempt short of the limit, so this sweep's failure exhausts it.
	id := seedRow(t, conn, "qrmenu.order.v1", "order.placed", 4, dueNow)

	// The relay calls WriteMessages twice here: once for the batch, which
	// fails and exhausts the attempt budget, and once for the DLQ, which
	// must succeed for the row to be retired.
	dlqPub := &sequencedPublisher{failFirst: errors.New("broker rejected the batch")}
	r := newWithPublisher(conn, Config{Source: "order.rpc", BatchSize: 50, MaxAttempts: 5}, dlqPub)

	if _, err := r.Sweep(context.Background()); err != nil {
		t.Fatalf("Sweep: %v", err)
	}

	var dlq *kafka.Message
	for _, m := range dlqPub.messages() {
		if strings.HasSuffix(m.Topic, ".dlq") {
			msg := m
			dlq = &msg
		}
	}
	if dlq == nil {
		t.Fatal("an exhausted row was not routed to the DLQ")
	}
	if dlq.Topic != "qrmenu.order.v1.dlq" {
		t.Errorf("DLQ topic = %q, want qrmenu.order.v1.dlq", dlq.Topic)
	}

	headers := map[string]string{}
	for _, h := range dlq.Headers {
		headers[h.Key] = string(h.Value)
	}
	if !strings.Contains(headers["dlq_reason"], "broker rejected") {
		t.Errorf("dlq_reason = %q, want the failure reason", headers["dlq_reason"])
	}
	if headers["dlq_source"] != "order.rpc" {
		t.Errorf("dlq_source = %q, want order.rpc", headers["dlq_source"])
	}
	if headers["dlq_attempts"] != "5" {
		t.Errorf("dlq_attempts = %q, want 5", headers["dlq_attempts"])
	}

	st := readRow(t, conn, id)
	if st.SentAt == nil {
		t.Error("a row routed to the DLQ should be retired, not retried forever")
	}
	if st.LastError == nil || !strings.Contains(*st.LastError, "sent to DLQ") {
		t.Errorf("last_error = %v, want it to record the DLQ routing", st.LastError)
	}
}

// TestSweepLeavesRowPendingIfDLQAlsoFails is the one case where a
// duplicate later is the right trade: never drop the event.
func TestSweepLeavesRowPendingIfDLQAlsoFails(t *testing.T) {
	conn := testConn(t)
	id := seedRow(t, conn, "qrmenu.order.v1", "order.placed", 4, dueNow)

	pub := &fakePublisher{err: errors.New("everything is down")}
	r := newWithPublisher(conn, Config{Source: "order.rpc", BatchSize: 50, MaxAttempts: 5}, pub)

	if _, err := r.Sweep(context.Background()); err != nil {
		t.Fatalf("Sweep: %v", err)
	}
	if st := readRow(t, conn, id); st.SentAt != nil {
		t.Error("a row whose DLQ write also failed must stay pending, not be dropped")
	}
}

func TestSweepRespectsBatchSize(t *testing.T) {
	conn := testConn(t)
	for i := 0; i < 5; i++ {
		seedRow(t, conn, "qrmenu.order.v1", "order.placed", 0, dueNow)
	}

	pub := &fakePublisher{}
	r := newWithPublisher(conn, Config{Source: "order.rpc", BatchSize: 2}, pub)

	n, err := r.Sweep(context.Background())
	if err != nil {
		t.Fatalf("Sweep: %v", err)
	}
	if n != 2 {
		t.Errorf("Sweep claimed %d rows, want 2", n)
	}
	if got := len(pub.messages()); got != 2 {
		t.Errorf("published %d messages, want 2", got)
	}
}

// sequencedPublisher fails only its first call, which is how the relay's
// two-phase batch-then-DLQ path is exercised.
type sequencedPublisher struct {
	mu        sync.Mutex
	failFirst error
	calls     int
	sent      []kafka.Message
}

func (p *sequencedPublisher) WriteMessages(_ context.Context, msgs ...kafka.Message) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.calls++
	if p.calls == 1 && p.failFirst != nil {
		return p.failFirst
	}
	p.sent = append(p.sent, msgs...)
	return nil
}

func (p *sequencedPublisher) Close() error { return nil }

func (p *sequencedPublisher) messages() []kafka.Message {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]kafka.Message(nil), p.sent...)
}
