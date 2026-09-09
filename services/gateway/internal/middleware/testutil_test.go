package middleware

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"math/big"
	"testing"
	"time"

	"github.com/menli02/QR-menu/proto/identity/v1"
	"github.com/menli02/QR-menu/services/identity/client/identityservice"

	"github.com/golang-jwt/jwt/v4"
	"google.golang.org/grpc"
)

// fakeIdentityService implements identityservice.IdentityService by
// embedding it (a nil interface value) and overriding only ListJWKS —
// every other method panics on call, which is fine: these tests only
// exercise the JWKS path.
type fakeIdentityService struct {
	identityservice.IdentityService
	jwks  *identityservice.JWKS
	err   error
	calls int
}

func (f *fakeIdentityService) ListJWKS(_ context.Context, _ *identityservice.ListJWKSRequest, _ ...grpc.CallOption) (*identityservice.JWKS, error) {
	f.calls++
	if f.err != nil {
		return nil, f.err
	}
	return f.jwks, nil
}

func generateTestKey(t *testing.T) *rsa.PrivateKey {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate RSA key: %v", err)
	}
	return key
}

// jwkFor mirrors identity/internal/authkey.PublicJWKFields's encoding
// (base64url, unsigned big-endian, RFC 7517 §6.3) — duplicated here
// because that package is Go-internal to the identity service and can't
// be imported from gateway's tests.
//
// Its return type is the proto package's JWK directly, not
// identityservice.JWK: the client shim only aliases the request/response
// types its interface methods use, not JWKS's nested element type.
func jwkFor(kid string, pub *rsa.PublicKey) *v1_identitypb.JWK {
	return &v1_identitypb.JWK{
		Kid: kid,
		Kty: "RSA",
		Use: "sig",
		Alg: "RS256",
		N:   base64.RawURLEncoding.EncodeToString(pub.N.Bytes()),
		E:   base64.RawURLEncoding.EncodeToString(big.NewInt(int64(pub.E)).Bytes()),
	}
}

// jwksResponse builds the *identityservice.JWKS a fakeIdentityService
// returns from ListJWKS, so call sites never need to spell out the JWK
// element type themselves.
func jwksResponse(keys ...*v1_identitypb.JWK) *identityservice.JWKS {
	return &identityservice.JWKS{Keys: keys}
}

func mintTestToken(t *testing.T, priv *rsa.PrivateKey, kid string, claims jwt.Claims) string {
	t.Helper()
	tok := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	tok.Header["kid"] = kid
	signed, err := tok.SignedString(priv)
	if err != nil {
		t.Fatalf("sign test token: %v", err)
	}
	return signed
}

func testStaffClaims(venueID, staffID, role string, ttl time.Duration) *staffJWTClaims {
	now := time.Now()
	return &staffJWTClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   staffID,
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
		},
		TokenType: tokenTypeStaff,
		VenueID:   venueID,
		Role:      role,
	}
}

func testGuestClaims(venueID, tableID, guestSessionID string, ttl time.Duration) *guestJWTClaims {
	now := time.Now()
	return &guestJWTClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   guestSessionID,
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
		},
		TokenType:      tokenTypeGuest,
		VenueID:        venueID,
		TableID:        tableID,
		GuestSessionID: guestSessionID,
	}
}

// jwksResponseFake is a one-key identity stand-in, for tests that only
// need a cache that resolves "kid-a".
type jwksResponseFake struct {
	fakeIdentityService
	key *rsa.PrivateKey
}

func (f *jwksResponseFake) ListJWKS(ctx context.Context, in *v1_identitypb.ListJWKSRequest, opts ...grpc.CallOption) (*v1_identitypb.JWKS, error) {
	return jwksResponse(jwkFor("kid-a", &f.key.PublicKey)), nil
}
