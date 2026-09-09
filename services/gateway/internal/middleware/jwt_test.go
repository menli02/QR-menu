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
	if err := verifyToken(context.Background(), cache, token, &claims, tokenTypeStaff); err != nil {
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
	if err := verifyToken(context.Background(), cache, token, &claims, tokenTypeStaff); err == nil {
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
	if err := verifyToken(context.Background(), cache, token, &claims, tokenTypeStaff); err == nil {
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
	if err := verifyToken(context.Background(), cache, token, &claims, tokenTypeStaff); err == nil {
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
	if err := verifyToken(context.Background(), cache, token, &claims, tokenTypeGuest); err != nil {
		t.Fatalf("verifyToken: %v", err)
	}
	if claims.VenueID != "venue-1" || claims.TableID != "table-1" || claims.GuestSessionID != "guest-session-1" {
		t.Fatalf("unexpected claims: %+v", claims)
	}
}

// TestVerifyToken_rejectsTheWrongTokenType is the regression test for a
// privilege escalation that was live until the `typ` claim was added.
//
// Staff and guest tokens are signed by the same key, and before this check
// a guest token parsed cleanly as staffJWTClaims — the struct simply found
// no `role` and left it empty. Every staff route requiring authentication
// but no particular role then accepted it. A guest token is what any
// customer gets by scanning the QR code on their table, so that exposed
// the live kitchen queue, the floor map with per-table totals, and every
// open service request to anyone sitting in the room.
func TestVerifyToken_rejectsTheWrongTokenType(t *testing.T) {
	key := generateTestKey(t)
	cache := newJWKSCache(&fakeIdentityService{
		jwks: jwksResponse(jwkFor("kid-a", &key.PublicKey)),
	}, time.Hour)

	guestToken := mintTestToken(t, key, "kid-a",
		testGuestClaims("venue-1", "table-1", "guest-session-1", 4*time.Hour))
	staffToken := mintTestToken(t, key, "kid-a",
		testStaffClaims("venue-1", "staff-1", "manager", time.Hour))

	t.Run("a guest token is not a staff token", func(t *testing.T) {
		var claims staffJWTClaims
		err := verifyToken(context.Background(), cache, guestToken, &claims, tokenTypeStaff)
		if err == nil {
			t.Fatal("a guest token was accepted as staff — this is the escalation the typ claim prevents")
		}
	})

	t.Run("a staff token is not a guest token", func(t *testing.T) {
		// Less dangerous in practice, but the same confusion: a staff
		// token carries no table_id, so it would produce a guest identity
		// bound to an empty table.
		var claims guestJWTClaims
		if err := verifyToken(context.Background(), cache, staffToken, &claims, tokenTypeGuest); err == nil {
			t.Fatal("a staff token was accepted as a guest token")
		}
	})

	t.Run("each type still works where it belongs", func(t *testing.T) {
		var staff staffJWTClaims
		if err := verifyToken(context.Background(), cache, staffToken, &staff, tokenTypeStaff); err != nil {
			t.Errorf("staff token rejected on a staff route: %v", err)
		}
		var guest guestJWTClaims
		if err := verifyToken(context.Background(), cache, guestToken, &guest, tokenTypeGuest); err != nil {
			t.Errorf("guest token rejected on a guest route: %v", err)
		}
	})
}

// TestVerifyToken_rejectsAMissingTokenType covers the fail-closed choice:
// a token minted before the claim existed has no `typ`, and treating that
// as "probably fine" would leave the hole open for the lifetime of every
// outstanding token.
func TestVerifyToken_rejectsAMissingTokenType(t *testing.T) {
	key := generateTestKey(t)
	cache := newJWKSCache(&jwksResponseFake{key: key}, time.Hour)

	legacy := testStaffClaims("venue-1", "staff-1", "manager", time.Hour)
	legacy.TokenType = "" // as an older identity would have minted it
	token := mintTestToken(t, key, "kid-a", legacy)

	var claims staffJWTClaims
	if err := verifyToken(context.Background(), cache, token, &claims, tokenTypeStaff); err == nil {
		t.Fatal("a token with no typ claim was accepted; it must fail closed")
	}
}
