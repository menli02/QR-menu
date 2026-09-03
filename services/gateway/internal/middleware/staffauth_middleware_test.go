package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/menli02/QR-menu/services/gateway/internal/errs"
)

func TestStaffAuthMiddleware_validToken_injectsClaimsAndCallsNext(t *testing.T) {
	key := generateTestKey(t)
	fake := &fakeIdentityService{jwks: jwksResponse(jwkFor("kid-a", &key.PublicKey))}
	m := NewStaffAuthMiddleware(fake, time.Hour)

	token := mintTestToken(t, key, "kid-a", testStaffClaims("venue-1", "staff-1", "manager", time.Hour))

	var gotClaims StaffClaims
	var nextCalled bool
	next := func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		gotClaims, _ = StaffClaimsFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/staff", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	m.Handle(next)(rec, req)

	if !nextCalled {
		t.Fatal("expected next to be called for a valid token")
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d", rec.Code)
	}
	if gotClaims != (StaffClaims{StaffID: "staff-1", VenueID: "venue-1", Role: "manager"}) {
		t.Fatalf("unexpected injected claims: %+v", gotClaims)
	}
}

func TestStaffAuthMiddleware_missingHeader_rejectsWithEnvelope(t *testing.T) {
	m := NewStaffAuthMiddleware(&fakeIdentityService{}, time.Hour)

	var nextCalled bool
	next := func(w http.ResponseWriter, r *http.Request) { nextCalled = true }

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/staff", nil)
	rec := httptest.NewRecorder()

	m.Handle(next)(rec, req)

	if nextCalled {
		t.Fatal("next must not be called without a token")
	}
	assertUnauthenticatedEnvelope(t, rec)
}

func TestStaffAuthMiddleware_invalidToken_rejectsWithEnvelope(t *testing.T) {
	key := generateTestKey(t)
	fake := &fakeIdentityService{jwks: jwksResponse(jwkFor("kid-a", &key.PublicKey))}
	m := NewStaffAuthMiddleware(fake, time.Hour)

	otherKey := generateTestKey(t)
	token := mintTestToken(t, otherKey, "kid-a", testStaffClaims("venue-1", "staff-1", "manager", time.Hour))

	var nextCalled bool
	next := func(w http.ResponseWriter, r *http.Request) { nextCalled = true }

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/staff", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	m.Handle(next)(rec, req)

	if nextCalled {
		t.Fatal("next must not be called for a token that fails verification")
	}
	assertUnauthenticatedEnvelope(t, rec)
}

// assertUnauthenticatedEnvelope checks the exact wire shape docs/TZ.md
// §8.1 promises, not just "some 4xx happened".
func assertUnauthenticatedEnvelope(t *testing.T, rec *httptest.ResponseRecorder) {
	t.Helper()
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
	var body errs.Body
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("response is not the documented error envelope: %v (body: %s)", err, rec.Body.String())
	}
	if body.Error.Code != errs.CodeUnauthenticated {
		t.Fatalf("expected code %q, got %q", errs.CodeUnauthenticated, body.Error.Code)
	}
}
