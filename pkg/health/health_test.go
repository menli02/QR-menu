package health

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

func probe(t *testing.T, s *Server) (int, string) {
	t.Helper()
	rec := httptest.NewRecorder()
	s.Handler(rec, httptest.NewRequest(http.MethodGet, "/readyz", nil))
	return rec.Code, rec.Body.String()
}

// TestNoChecksIsReady documents the chosen default for a service with no
// registered dependencies: ready.
//
// The opposite convention (go-zero's own combo manager reports not-ready
// with zero probes) makes sense there, where an empty set means "nothing
// has started yet". Here an empty set means the operator registered
// nothing, and failing a startup probe forever over that would be a
// crash-loop with no diagnostic.
func TestNoChecksIsReady(t *testing.T) {
	code, _ := probe(t, NewServer(0))
	if code != http.StatusOK {
		t.Errorf("code = %d, want 200 for a server with no checks", code)
	}
}

func TestAllChecksPassing(t *testing.T) {
	s := NewServer(0)
	s.Register("postgres", func(context.Context) error { return nil })
	s.Register("kafka", func(context.Context) error { return nil })

	code, body := probe(t, s)
	if code != http.StatusOK {
		t.Errorf("code = %d, want 200", code)
	}
	if !strings.Contains(body, "ready") {
		t.Errorf("body = %q", body)
	}
}

// TestFailureNamesTheDependency is the difference between a useful probe
// and a useless one at three in the morning.
func TestFailureNamesTheDependency(t *testing.T) {
	s := NewServer(0)
	s.Register("postgres", func(context.Context) error { return errors.New("connection refused") })
	s.Register("kafka", func(context.Context) error { return nil })

	code, body := probe(t, s)
	if code != http.StatusServiceUnavailable {
		t.Fatalf("code = %d, want 503", code)
	}
	if !strings.Contains(body, "postgres") {
		t.Errorf("body does not name the failing dependency: %q", body)
	}
	if !strings.Contains(body, "connection refused") {
		t.Errorf("body does not carry the cause: %q", body)
	}
	if strings.Contains(body, "kafka") {
		t.Errorf("body mentions a dependency that was fine: %q", body)
	}
}

func TestAllFailuresAreReported(t *testing.T) {
	s := NewServer(0)
	s.Register("postgres", func(context.Context) error { return errors.New("down") })
	s.Register("kafka", func(context.Context) error { return errors.New("also down") })

	code, body := probe(t, s)
	if code != http.StatusServiceUnavailable {
		t.Fatalf("code = %d, want 503", code)
	}
	// Both, not just the first: an operator fixing one and finding the
	// other still broken has wasted a deploy cycle.
	if !strings.Contains(body, "postgres") || !strings.Contains(body, "kafka") {
		t.Errorf("body should name every failure: %q", body)
	}
}

// TestFailureOrderIsStable keeps the same outage reading the same way each
// time it is probed.
func TestFailureOrderIsStable(t *testing.T) {
	s := NewServer(0)
	for _, name := range []string{"zebra", "alpha", "middle"} {
		s.Register(name, func(context.Context) error { return errors.New("down") })
	}

	_, first := probe(t, s)
	for i := 0; i < 20; i++ {
		if _, body := probe(t, s); body != first {
			t.Fatalf("probe output varies between calls:\n%q\nvs\n%q", first, body)
		}
	}
}

// TestChecksRunInParallel: three 100ms checks must take ~100ms, not 300ms.
// Serial evaluation would make the probe's latency the sum of its
// dependencies' and eventually trip the kubelet's own timeout.
func TestChecksRunInParallel(t *testing.T) {
	s := NewServer(0)
	for _, name := range []string{"a", "b", "c"} {
		s.Register(name, func(ctx context.Context) error {
			select {
			case <-time.After(100 * time.Millisecond):
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		})
	}

	start := time.Now()
	code, _ := probe(t, s)
	elapsed := time.Since(start)

	if code != http.StatusOK {
		t.Fatalf("code = %d, want 200", code)
	}
	if elapsed > 250*time.Millisecond {
		t.Errorf("three parallel 100ms checks took %s — they look serial", elapsed)
	}
}

// TestSlowCheckIsCutOffByTheTimeout: the probe must decide for itself
// rather than hanging until the kubelet gives up, which produces no
// diagnostic at all.
func TestSlowCheckIsCutOffByTheTimeout(t *testing.T) {
	s := NewServer(0)
	s.timeout = 50 * time.Millisecond
	s.Register("slow", func(ctx context.Context) error {
		<-ctx.Done()
		return ctx.Err()
	})

	start := time.Now()
	code, body := probe(t, s)
	elapsed := time.Since(start)

	if code != http.StatusServiceUnavailable {
		t.Errorf("code = %d, want 503", code)
	}
	if elapsed > time.Second {
		t.Errorf("probe took %s — it did not enforce its own timeout", elapsed)
	}
	if !strings.Contains(body, "slow") {
		t.Errorf("body should name the check that timed out: %q", body)
	}
}

func TestRecoversWhenTheDependencyReturns(t *testing.T) {
	var down atomic.Bool
	down.Store(true)

	s := NewServer(0)
	s.Register("postgres", func(context.Context) error {
		if down.Load() {
			return errors.New("connection refused")
		}
		return nil
	})

	if code, _ := probe(t, s); code != http.StatusServiceUnavailable {
		t.Fatalf("code = %d, want 503 while down", code)
	}
	down.Store(false)
	if code, _ := probe(t, s); code != http.StatusOK {
		t.Errorf("code = %d, want 200 once recovered — the probe must not latch", code)
	}
}

func TestTCPCheck(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	defer srv.Close()
	addr := strings.TrimPrefix(srv.URL, "http://")

	if err := TCPCheck(addr)(context.Background()); err != nil {
		t.Errorf("TCPCheck against a listening port: %v", err)
	}

	// Port 1 on loopback: reserved and reliably closed.
	err := TCPCheck("127.0.0.1:1")(context.Background())
	if err == nil {
		t.Error("TCPCheck against a closed port should fail")
	} else if !strings.Contains(err.Error(), "127.0.0.1:1") {
		t.Errorf("error should name the unreachable address: %v", err)
	}
}

// fakeConn stands in for a go-zero sqlx.SqlConn.
type fakeConn struct {
	db  *sql.DB
	err error
}

func (f fakeConn) RawDB() (*sql.DB, error) { return f.db, f.err }

// TestPostgresCheckReportsAMissingPool is the misconfigured-DSN case: the
// pool cannot even be constructed, and the check must say so rather than
// dereference a nil handle.
func TestPostgresCheckReportsAMissingPool(t *testing.T) {
	err := PostgresCheck(fakeConn{err: errors.New("malformed dsn")})(context.Background())
	if err == nil {
		t.Fatal("a pool that cannot be built should fail the check")
	}
	if !strings.Contains(err.Error(), "malformed dsn") {
		t.Errorf("error should carry the cause: %v", err)
	}
}

// TestPostgresCheckPingsThePool uses a real *sql.DB whose driver is
// registered but whose server does not exist, so Ping genuinely fails —
// which is the production symptom of an unreachable database.
func TestPostgresCheckPingsThePool(t *testing.T) {
	db, err := sql.Open("pgx", "postgres://nobody:nobody@127.0.0.1:1/nothing?sslmode=disable")
	if err != nil {
		t.Skipf("pgx driver unavailable: %v", err)
	}
	defer func() { _ = db.Close() }()

	checkErr := PostgresCheck(fakeConn{db: db})(context.Background())
	if checkErr == nil {
		t.Fatal("pinging an unreachable database should fail the check")
	}
	if !strings.Contains(checkErr.Error(), "unreachable") {
		t.Errorf("error should be labelled as unreachable: %v", checkErr)
	}
}
