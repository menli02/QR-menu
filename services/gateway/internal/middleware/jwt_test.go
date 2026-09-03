package middleware

import (
	"context"
	"testing"
	"time"
)

func TestVerifyToken_staffRoundTrip(t *testing.T) {
	key := generateTestKey(t)
	cache := newJWKSCache(&fakeIdentityService{
		jwks: jwksResponse(jwkFor("kid-a", &key.PublicKey)),
	}, time.Hour)

	token := mintTestToken(t, key, "kid-a", testStaffClaims("venue-1", "staff-1", "manager", time.Hour))

	var claims staffJWTClaims
	if err := verifyToken(context.Background(), cache, token, &claims); err != nil {
		t.Fatalf("verifyToken: %v", err)
	}
	if claims.Subject != "staff-1" || claims.VenueID != "venue-1" || claims.Role != "manager" {
		t.Fatalf("unexpected claims: %+v", claims)
	}
}

func TestVerifyToken_wrongKeyRejected(t *testing.T) {
	signingKey := generateTestKey(t)
	publishedKey := generateTestKey(t) // JWKS advertises a DIFFERENT key under the same kid
	cache := newJWKSCache(&fakeIdentityService{
		jwks: jwksResponse(jwkFor("kid-a", &publishedKey.PublicKey)),
	}, time.Hour)

	token := mintTestToken(t, signingKey, "kid-a", testStaffClaims("venue-1", "staff-1", "manager", time.Hour))

	var claims staffJWTClaims
	if err := verifyToken(context.Background(), cache, token, &claims); err == nil {
		t.Fatal("expected verification to fail: token was signed with a key JWKS never published")
	}
}

func TestVerifyToken_expiredRejected(t *testing.T) {
	key := generateTestKey(t)
	cache := newJWKSCache(&fakeIdentityService{
		jwks: jwksResponse(jwkFor("kid-a", &key.PublicKey)),
	}, time.Hour)

	token := mintTestToken(t, key, "kid-a", testStaffClaims("venue-1", "staff-1", "manager", -time.Second))

	var claims staffJWTClaims
	if err := verifyToken(context.Background(), cache, token, &claims); err == nil {
		t.Fatal("expected an already-expired token to fail verification")
	}
}

func TestVerifyToken_unknownKidRejected(t *testing.T) {
	key := generateTestKey(t)
	cache := newJWKSCache(&fakeIdentityService{
		jwks: jwksResponse(jwkFor("kid-a", &key.PublicKey)),
	}, time.Hour)

	// Signed with a key that was never published under any kid.
	otherKey := generateTestKey(t)
	token := mintTestToken(t, otherKey, "kid-does-not-exist", testStaffClaims("venue-1", "staff-1", "manager", time.Hour))

	var claims staffJWTClaims
	if err := verifyToken(context.Background(), cache, token, &claims); err == nil {
		t.Fatal("expected verification to fail for an unpublished kid")
	}
}

func TestVerifyToken_guestRoundTrip(t *testing.T) {
	key := generateTestKey(t)
	cache := newJWKSCache(&fakeIdentityService{
		jwks: jwksResponse(jwkFor("kid-a", &key.PublicKey)),
	}, time.Hour)

	token := mintTestToken(t, key, "kid-a", testGuestClaims("venue-1", "table-1", "guest-session-1", 4*time.Hour))

	var claims guestJWTClaims
	if err := verifyToken(context.Background(), cache, token, &claims); err != nil {
		t.Fatalf("verifyToken: %v", err)
	}
	if claims.VenueID != "venue-1" || claims.TableID != "table-1" || claims.GuestSessionID != "guest-session-1" {
		t.Fatalf("unexpected claims: %+v", claims)
	}
}
