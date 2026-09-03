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
	VenueID string `json:"venue_id"`
	Role    string `json:"role"`
}

type guestJWTClaims struct {
	jwt.RegisteredClaims
	VenueID        string `json:"venue_id"`
	TableID        string `json:"table_id"`
	GuestSessionID string `json:"guest_session_id"`
}

// verifyToken parses tokenString into claims, resolving the verification
// key from cache by the token header's kid. Rejects anything not signed
// with RSA — without this check, a token whose header simply claims
// alg=HS256 (the "none"/alg-confusion family of JWT bugs) could otherwise
// get verified using an RSA public key value as if it were an HMAC
// secret.
func verifyToken(ctx context.Context, cache *jwksCache, tokenString string, claims jwt.Claims) error {
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
	return err
}
