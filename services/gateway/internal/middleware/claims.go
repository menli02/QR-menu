package middleware

import "context"

type ctxKey int

const (
	staffClaimsKey ctxKey = iota
	guestClaimsKey
)

// StaffClaims are the identity claims extracted from a verified staff JWT
// by StaffAuthMiddleware. Per-route role checks (admin/manager/waiter/
// cook) are the logic layer's job (docs/TZ.md §8.2, §11.3) — this
// middleware only establishes who the caller is, not what they may do.
type StaffClaims struct {
	StaffID string
	VenueID string
	Role    string
}

// GuestClaims are the identity claims extracted from a verified guest JWT
// by GuestAuthMiddleware.
type GuestClaims struct {
	VenueID        string
	TableID        string
	GuestSessionID string
}

// WithStaffClaims attaches verified staff claims to a context.
//
// Exported because the auth middleware is not the only place that
// verifies a token: the WebSocket endpoints take theirs in the first
// frame rather than an Authorization header (docs/TZ.md §8.1 —
// "Token in the first message, not the query string", so it never lands
// in a proxy access log), and need to build the same context afterwards.
// Everything downstream then reads claims the one way.
func WithStaffClaims(ctx context.Context, c StaffClaims) context.Context {
	return context.WithValue(ctx, staffClaimsKey, c)
}

func withStaffClaims(ctx context.Context, c StaffClaims) context.Context {
	return WithStaffClaims(ctx, c)
}

// StaffClaimsFromContext returns the claims StaffAuthMiddleware injected.
// ok is false if called on a request that never went through it.
func StaffClaimsFromContext(ctx context.Context) (StaffClaims, bool) {
	c, ok := ctx.Value(staffClaimsKey).(StaffClaims)
	return c, ok
}

// WithGuestClaims attaches verified guest claims to a context. See
// WithStaffClaims for why this is exported.
func WithGuestClaims(ctx context.Context, c GuestClaims) context.Context {
	return context.WithValue(ctx, guestClaimsKey, c)
}

func withGuestClaims(ctx context.Context, c GuestClaims) context.Context {
	return WithGuestClaims(ctx, c)
}

// GuestClaimsFromContext returns the claims GuestAuthMiddleware injected.
func GuestClaimsFromContext(ctx context.Context) (GuestClaims, bool) {
	c, ok := ctx.Value(guestClaimsKey).(GuestClaims)
	return c, ok
}
