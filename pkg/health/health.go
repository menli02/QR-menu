// Package health exposes a dependency-aware readiness endpoint.
//
// # Why this exists alongside go-zero's /healthz
//
// go-zero already serves /healthz and grpc.health.v1.Health, and both go
// green the moment the server binds its port. That is a genuine liveness
// signal — the process is up and accepting connections — but it says
// nothing about whether the service can actually do its job. A pod whose
// Postgres credentials are wrong reports SERVING and then fails every
// request with a 500.
//
// So this package adds a second, narrower question: are this pod's
// critical dependencies reachable right now? The answer drives the
// Kubernetes *startup* probe.
//
// # Why startup, and not readiness
//
// This is the part worth being explicit about, because the obvious choice
// is wrong.
//
// Wiring a shared-database check to the readiness probe looks right and
// fails badly: every replica shares one Postgres, so a database blip makes
// every pod unready at the same moment, the Service loses all its
// endpoints, and the cluster turns a degraded service into a total
// blackout — with no healthy pod anywhere to shift traffic to. Worse, the
// pods can no longer return honest 503s with a trace id, because nothing
// reaches them at all.
//
// Used as a *startup* probe, the same check earns its keep: a newly
// scheduled pod does not receive traffic until it has proved it can talk
// to its database, which catches the failure this actually guards against
// — a bad rollout, a wrong secret, an unmigrated schema — without coupling
// the fate of every running pod to one dependency.
//
// Liveness and readiness therefore both point at go-zero's /healthz, which
// depends on nothing external and cannot cascade.
package health

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/zeromicro/go-zero/core/logx"
)

// CheckFunc reports whether one dependency is usable. It must respect the
// context deadline: a probe that hangs is worse than one that fails,
// because the kubelet's own timeout then decides the outcome with no
// diagnostic.
type CheckFunc func(ctx context.Context) error

// DefaultTimeout bounds a whole probe evaluation. Comfortably under the
// 3s timeout the manifests set, so a slow dependency is reported as a
// failure by us — with a name attached — rather than as an opaque kubelet
// timeout.
const DefaultTimeout = 2 * time.Second

// Server serves GET /readyz on its own tiny listener.
//
// Its own listener, rather than a route on an existing server, because the
// four services do not share one: three are gRPC and have no HTTP surface,
// and go-zero's admin server owns its mux. One uniform port across every
// service keeps the Kubernetes manifests identical.
type Server struct {
	addr    string
	timeout time.Duration

	mu     sync.RWMutex
	checks map[string]CheckFunc

	srv  *http.Server
	done chan struct{}
}

func NewServer(port int) *Server {
	return &Server{
		addr:    fmt.Sprintf(":%d", port),
		timeout: DefaultTimeout,
		checks:  make(map[string]CheckFunc),
		done:    make(chan struct{}),
	}
}

// Register adds a named dependency check. The name appears in the failure
// response, which is the difference between "readiness failed" and
// "readiness failed: postgres" at three in the morning.
func (s *Server) Register(name string, check CheckFunc) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.checks[name] = check
}

// Handler evaluates every check and reports 200 or 503.
//
// Checks run in parallel: they are independent network round trips, and
// running them in series would make the probe's latency the sum of its
// dependencies' rather than the slowest one.
func (s *Server) Handler(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), s.timeout)
	defer cancel()

	s.mu.RLock()
	checks := make(map[string]CheckFunc, len(s.checks))
	for name, fn := range s.checks {
		checks[name] = fn
	}
	s.mu.RUnlock()

	type result struct {
		name string
		err  error
	}
	results := make(chan result, len(checks))
	for name, fn := range checks {
		go func(name string, fn CheckFunc) {
			results <- result{name: name, err: fn(ctx)}
		}(name, fn)
	}

	var failures []string
	for range checks {
		r := <-results
		if r.err != nil {
			failures = append(failures, fmt.Sprintf("%s: %v", r.name, r.err))
		}
	}

	if len(failures) == 0 {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ready\n"))
		return
	}

	// Sorted so the same failure reads the same way every time — a probe
	// response that reorders itself is needlessly hard to diff in logs.
	sort.Strings(failures)
	logx.Errorf("readiness check failed: %s", strings.Join(failures, "; "))
	w.WriteHeader(http.StatusServiceUnavailable)
	_, _ = fmt.Fprintf(w, "not ready\n%s\n", strings.Join(failures, "\n"))
}

// Start runs the listener until Stop. It blocks, matching go-zero's
// service.Service contract, so it can join a ServiceGroup.
func (s *Server) Start() {
	defer close(s.done)

	mux := http.NewServeMux()
	mux.HandleFunc("/readyz", s.Handler)

	s.srv = &http.Server{
		Addr:              s.addr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	logx.Infof("readiness server listening on %s/readyz", s.addr)
	if err := s.srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		logx.Errorf("readiness server failed: %v", err)
	}
}

func (s *Server) Stop() {
	if s.srv != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = s.srv.Shutdown(ctx)
	}
	<-s.done
}

// RawDB is the one method this package needs from a connection pool,
// matching go-zero's sqlx.SqlConn. Declared as a local interface so
// pkg/health depends on no particular driver or ORM.
type RawDB interface {
	RawDB() (*sql.DB, error)
}

// PostgresCheck adapts a go-zero sqlx.SqlConn to a CheckFunc.
//
// A ping, not a query: this asks "can the pool hand me a working
// connection", which is exactly the startup failure worth catching —
// wrong host, wrong credentials, database not created. Running real SQL
// here would start testing the schema, which is the migration job's
// business, not the pod's.
//
// Resolving the pool inside the check rather than at construction is
// deliberate: postgres.New dials lazily, so at wiring time there may be no
// *sql.DB yet, and a check captured too early would report a stale handle.
func PostgresCheck(conn RawDB) CheckFunc {
	return func(ctx context.Context) error {
		db, err := conn.RawDB()
		if err != nil {
			return fmt.Errorf("no connection pool: %w", err)
		}
		if err := db.PingContext(ctx); err != nil {
			return fmt.Errorf("unreachable: %w", err)
		}
		return nil
	}
}

// TCPCheck reports whether a plain TCP endpoint accepts connections. Used
// for dependencies with no richer client at hand — etcd, a Kafka broker —
// where "the socket opens" is the honest limit of what can be asserted
// cheaply.
func TCPCheck(address string) CheckFunc {
	return func(ctx context.Context) error {
		var d net.Dialer
		conn, err := d.DialContext(ctx, "tcp", address)
		if err != nil {
			return fmt.Errorf("cannot reach %s: %w", address, err)
		}
		return conn.Close()
	}
}
