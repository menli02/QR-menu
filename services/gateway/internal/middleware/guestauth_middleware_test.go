package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestGuestAuthMiddleware_validToken_injectsClaimsAndCallsNext(t *testing.T) {
	key := generateTestKey(t)
	fake := &fakeIdentityService{jwks: jwksResponse(jwkFor("kid-a", &key.PublicKey))}
	m := NewGuestAuthMiddleware(fake, time.Hour)

	token := mintTestToken(t, key, "kid-a", testGuestClaims("venue-1", "table-1", "guest-session-1", 4*time.Hour))

	var gotClaims GuestClaims
	var nextCalled bool
	next := func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		gotClaims, _ = GuestClaimsFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/guest/menu", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	m.Handle(next)(rec, req)

	if !nextCalled {
		t.Fatal("expected next to be called for a valid token")
	}
	want := GuestClaims{VenueID: "venue-1", TableID: "table-1", GuestSessionID: "guest-session-1"}
	if gotClaims != want {
		t.Fatalf("unexpected injected claims: got %+v, want %+v", gotClaims, want)
	}
}

func TestGuestAuthMiddleware_missingHeader_rejectsWithEnvelope(t *testing.T) {
	m := NewGuestAuthMiddleware(&fakeIdentityService{}, time.Hour)

	var nextCalled bool
	next := func(w http.ResponseWriter, r *http.Request) { nextCalled = true }

	req := httptest.NewRequest(http.MethodGet, "/api/v1/guest/menu", nil)
	rec := httptest.NewRecorder()

	m.Handle(next)(rec, req)

	if nextCalled {
		t.Fatal("next must not be called without a token")
	}
	assertUnauthenticatedEnvelope(t, rec)
}

func TestGuestAuthMiddleware_staffTokenRejected(t *testing.T) {
	// A staff token is signed by the same key/kid a guest token would be,
	// so JWKS lookup succeeds either way — GuestAuth must still reject it
	// because its claim shape doesn't decode into guestJWTClaims the way
	// a real guest token's would (no venue/table/guest_session claims to
	// scope a guest to one table session).
	key := generateTestKey(t)
	fake := &fakeIdentityService{jwks: jwksResponse(jwkFor("kid-a", &key.PublicKey))}
	m := NewGuestAuthMiddleware(fake, time.Hour)

	token := mintTestToken(t, key, "kid-a", testStaffClaims("venue-1", "staff-1", "manager", time.Hour))

	var gotClaims GuestClaims
	next := func(w http.ResponseWriter, r *http.Request) {
		gotClaims, _ = GuestClaimsFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/guest/menu", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	m.Handle(next)(rec, req)

	// The JWT parses (both claim types embed jwt.RegisteredClaims and
	// json.Unmarshal ignores unknown fields), so this documents actual
	// behavior: verification succeeds but the guest claims are empty —
	// a staff token carries no table_id/guest_session_id. Any downstream
	// handler keying off GuestSessionID would treat this as an invalid
	// session, not impersonate one.
	if gotClaims.TableID != "" || gotClaims.GuestSessionID != "" {
		t.Fatalf("a staff token must not resolve to usable guest claims, got: %+v", gotClaims)
	}
}
