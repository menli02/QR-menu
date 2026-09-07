package reqctx

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

// run drives the middleware and hands the resulting context to inspect.
func run(t *testing.T, mutate func(*http.Request), inspect func(context.Context)) {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/guest/orders", nil)
	req.RemoteAddr = "192.0.2.10:54321"
	if mutate != nil {
		mutate(req)
	}
	called := false
	handler := Middleware(func(_ http.ResponseWriter, r *http.Request) {
		called = true
		inspect(r.Context())
	})
	handler(httptest.NewRecorder(), req)
	if !called {
		t.Fatal("the middleware did not call the next handler")
	}
}

func TestIdempotencyKeyIsLifted(t *testing.T) {
	run(t, func(r *http.Request) {
		r.Header.Set(IdempotencyHeader, "b0a1c2d3-e4f5-6789-abcd-ef0123456789")
	}, func(ctx context.Context) {
		if got := IdempotencyKey(ctx); got != "b0a1c2d3-e4f5-6789-abcd-ef0123456789" {
			t.Errorf("IdempotencyKey = %q", got)
		}
	})
}

// TestMissingIdempotencyKeyIsEmpty matters because the middleware
// deliberately does not enforce the header — most routes are GETs. The
// handlers that need one check for "" themselves.
func TestMissingIdempotencyKeyIsEmpty(t *testing.T) {
	run(t, nil, func(ctx context.Context) {
		if got := IdempotencyKey(ctx); got != "" {
			t.Errorf("IdempotencyKey = %q, want empty", got)
		}
	})
}

func TestIdempotencyKeyOnABareContext(t *testing.T) {
	// A logic function called outside a request (a test, a background
	// job) must get "" rather than panic on the type assertion.
	if got := IdempotencyKey(context.Background()); got != "" {
		t.Errorf("IdempotencyKey = %q, want empty", got)
	}
	if got := ClientIP(context.Background()); got != "" {
		t.Errorf("ClientIP = %q, want empty", got)
	}
}

func TestClientIP(t *testing.T) {
	cases := []struct {
		name    string
		headers map[string]string
		remote  string
		want    string
	}{
		{
			name:   "falls back to RemoteAddr",
			remote: "192.0.2.10:54321",
			want:   "192.0.2.10",
		},
		{
			name:    "prefers the first X-Forwarded-For hop",
			headers: map[string]string{"X-Forwarded-For": "203.0.113.7, 10.0.0.1, 10.0.0.2"},
			remote:  "10.0.0.2:443",
			want:    "203.0.113.7",
		},
		{
			name:    "trims whitespace",
			headers: map[string]string{"X-Forwarded-For": "  203.0.113.7  ,10.0.0.1"},
			want:    "203.0.113.7",
		},
		{
			name:    "single-hop X-Forwarded-For",
			headers: map[string]string{"X-Forwarded-For": "203.0.113.7"},
			want:    "203.0.113.7",
		},
		{
			name:    "X-Real-Ip when there is no XFF",
			headers: map[string]string{"X-Real-Ip": "203.0.113.9"},
			want:    "203.0.113.9",
		},
		{
			name:   "IPv6 RemoteAddr keeps its colons",
			remote: "[2001:db8::1]:54321",
			want:   "2001:db8::1",
		},
		{
			name:   "RemoteAddr with no port",
			remote: "192.0.2.10",
			want:   "192.0.2.10",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			run(t, func(r *http.Request) {
				if tc.remote != "" {
					r.RemoteAddr = tc.remote
				}
				for k, v := range tc.headers {
					r.Header.Set(k, v)
				}
			}, func(ctx context.Context) {
				if got := ClientIP(ctx); got != tc.want {
					t.Errorf("ClientIP = %q, want %q", got, tc.want)
				}
			})
		})
	}
}
