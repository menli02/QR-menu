// Package reqctx carries per-request HTTP details that the logic layer
// needs but goctl's generated signatures don't pass.
//
// A logic function receives only (ctx, *types.XxxReq) — no *http.Request
// — so a header like Idempotency-Key has nowhere to arrive. Rather than
// hand-editing generated handlers (which regeneration would overwrite),
// a global middleware lifts the headers we need into the context, and
// this package is the only place that knows the keys.
package reqctx

import (
	"context"
	"net"
	"net/http"
	"strings"

	"github.com/zeromicro/go-zero/rest"
)

type ctxKey int

const (
	idempotencyKeyKey ctxKey = iota
	clientIPKey
)

// IdempotencyHeader is §7.4's required header on state-changing public
// endpoints.
const IdempotencyHeader = "Idempotency-Key"

// Middleware lifts the request details this package exposes into the
// context. Registered globally in gateway.go, before routing, so every
// handler sees them.
func Middleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		if key := r.Header.Get(IdempotencyHeader); key != "" {
			ctx = context.WithValue(ctx, idempotencyKeyKey, key)
		}
		ctx = context.WithValue(ctx, clientIPKey, clientIP(r))
		next(w, r.WithContext(ctx))
	}
}

var _ rest.Middleware = Middleware

// IdempotencyKey returns the client's Idempotency-Key, or "" if absent.
// A logic function that requires one should reject the empty case itself
// with VALIDATION_FAILED — the middleware doesn't enforce it, because
// most routes don't need one and a blanket rule would reject every GET.
func IdempotencyKey(ctx context.Context) string {
	key, _ := ctx.Value(idempotencyKeyKey).(string)
	return key
}

// ClientIP is the caller's address, for per-IP rate limiting on the
// unauthenticated guest session exchange (§8.1: "Rate limited per IP").
func ClientIP(ctx context.Context) string {
	ip, _ := ctx.Value(clientIPKey).(string)
	return ip
}

// clientIP prefers the first X-Forwarded-For hop, since the gateway sits
// behind an ingress in every deployment that matters.
//
// This is trusted input only because nothing outside the cluster can
// reach the gateway directly — the ingress overwrites the header. If that
// ever stops being true, a spoofed XFF defeats the rate limit, so the
// assumption is written down here rather than left implicit.
func clientIP(r *http.Request) string {
	if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
		first, _, _ := strings.Cut(fwd, ",")
		return strings.TrimSpace(first)
	}
	if real := r.Header.Get("X-Real-Ip"); real != "" {
		return strings.TrimSpace(real)
	}
	// RemoteAddr is host:port; a bare address (no port) makes
	// SplitHostPort fail, in which case the address is already what we
	// want.
	if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		return host
	}
	return r.RemoteAddr
}
