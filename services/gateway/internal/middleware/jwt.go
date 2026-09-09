package middleware

import (
	"context"
	"fmt"

	"github.com/golang-jwt/jwt/v4"
)

// staffJWTClaims and guestJWTClaims mirror the JSON claim shapes minted by
// identity/internal/authkey — that package is Go-internal to the identity
// service and cannot be imported from here, so this is a deliberate
// wire-format duplication, not a shared type. The signed token itself
// (verified against ListJWKS), not a shared Go type, is the contract
// between the two services (docs/TZ.md §8.2).
type staffJWTClaims struct {
	jwt.RegisteredClaims
	TokenType string `json:"typ"`
	VenueID   string `json:"venue_id"`
	Role      string `json:"role"`
}

type guestJWTClaims struct {
	jwt.RegisteredClaims
	TokenType      string `json:"typ"`
	VenueID        string `json:"venue_id"`
	TableID        string `json:"table_id"`
	GuestSessionID string `json:"guest_session_id"`
}

// Token type discriminators, mirroring identity's authkey constants.
//
// Checking these is not a formality. Staff and guest tokens are signed by
// the same key, so before the `typ` claim existed a guest token parsed
// cleanly as staffJWTClaims — the struct simply found no `role` and left
// it empty. Every staff route that requires authentication but no
// particular role therefore accepted a guest token, and a guest token is
// something anyone who sits down and scans the QR code on the table can
// get. That opened the live kitchen queue, the floor map with per-table
// totals, and every open service request to any customer in the room.
//
// Found by probing the qr.pdf route: a guest token returned 403 rather
// than 401, which meant the staff middleware had accepted it and only the
// admin role check turned it away.
const (
	tokenTypeStaff = "staff"
	tokenTypeGuest = "guest"
)

// verifyToken parses tokenString into claims, resolving the verification
// key from cache by the token header's kid. Rejects anything not signed
// with RSA — without this check, a token whose header simply claims
// alg=HS256 (the "none"/alg-confusion family of JWT bugs) could otherwise
// get verified using an RSA public key value as if it were an HMAC
// secret.
// It also enforces the token's `typ`: see the constants above for the
// privilege escalation that omitting this check allows.
func verifyToken(ctx context.Context, cache *jwksCache, tokenString string, claims jwt.Claims, wantType string) error {
	_, err := jwt.ParseWithClaims(tokenString, claims, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("unexpected signing method %v", t.Header["alg"])
		}
		kid, ok := t.Header["kid"].(string)
		if !ok || kid == "" {
			return nil, fmt.Errorf("token has no kid header")
		}
		return cache.get(ctx, kid)
	})
	if err != nil {
		return err
	}

	// Checked after the signature, so an attacker learns nothing from the
	// ordering, and required rather than defaulted: a token minted before
	// the claim existed has no `typ` and must be rejected, not waved
	// through. Access tokens are short-lived (15 min staff, 4h guest), so
	// the cost of that is one round of re-authentication at deploy.
	got := tokenTypeOf(claims)
	if got != wantType {
		return fmt.Errorf("token type %q is not valid here, want %q", got, wantType)
	}
	return nil
}

func tokenTypeOf(claims jwt.Claims) string {
	switch c := claims.(type) {
	case *staffJWTClaims:
		return c.TokenType
	case *guestJWTClaims:
		return c.TokenType
	default:
		return ""
	}
}
