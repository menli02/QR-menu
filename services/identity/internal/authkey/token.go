package authkey

import (
	"crypto/rsa"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v4"
)

// StaffClaims are embedded in a staff access token. The subject is the
// staff id.
type StaffClaims struct {
	jwt.RegisteredClaims
	VenueID string `json:"venue_id"`
	Role    string `json:"role"`
}

// GuestClaims are embedded in an anonymous guest token (docs/TZ.md FR-O1):
// bound to one venue, table and guest session. The subject is the guest
// session id.
type GuestClaims struct {
	jwt.RegisteredClaims
	VenueID        string `json:"venue_id"`
	TableID        string `json:"table_id"`
	GuestSessionID string `json:"guest_session_id"`
}

// MintStaff signs a staff access token with priv, identified in its
// header by kid (so a verifier can pick the right JWK out of ListJWKS).
func MintStaff(priv *rsa.PrivateKey, kid, staffID, venueID, role string, ttl time.Duration) (token string, expiresAt time.Time, err error) {
	now := time.Now()
	expiresAt = now.Add(ttl)
	claims := StaffClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   staffID,
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(expiresAt),
		},
		VenueID: venueID,
		Role:    role,
	}
	token, err = sign(priv, kid, claims)
	return token, expiresAt, err
}

// MintGuest signs an anonymous guest token with priv.
func MintGuest(priv *rsa.PrivateKey, kid, venueID, tableID, guestSessionID string, ttl time.Duration) (token string, expiresAt time.Time, err error) {
	now := time.Now()
	expiresAt = now.Add(ttl)
	claims := GuestClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   guestSessionID,
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(expiresAt),
		},
		VenueID:        venueID,
		TableID:        tableID,
		GuestSessionID: guestSessionID,
	}
	token, err = sign(priv, kid, claims)
	return token, expiresAt, err
}

func sign(priv *rsa.PrivateKey, kid string, claims jwt.Claims) (string, error) {
	t := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	t.Header["kid"] = kid
	return t.SignedString(priv)
}

// ParseStaff verifies a staff access token's signature against pub and
// returns its claims. Identity itself never calls this in production —
// gateway verifies access tokens offline against ListJWKS (docs/TZ.md
// §8.2) — it exists so tests can prove a token minted here actually
// verifies against the same public key JWKS would publish.
func ParseStaff(tokenString string, pub *rsa.PublicKey) (*StaffClaims, error) {
	claims := &StaffClaims{}
	if err := parse(tokenString, pub, claims); err != nil {
		return nil, err
	}
	return claims, nil
}

// ParseGuest is ParseStaff's guest-token counterpart.
func ParseGuest(tokenString string, pub *rsa.PublicKey) (*GuestClaims, error) {
	claims := &GuestClaims{}
	if err := parse(tokenString, pub, claims); err != nil {
		return nil, err
	}
	return claims, nil
}

func parse(tokenString string, pub *rsa.PublicKey, claims jwt.Claims) error {
	_, err := jwt.ParseWithClaims(tokenString, claims, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("authkey: unexpected signing method %v", t.Header["alg"])
		}
		return pub, nil
	})
	return err
}
