package model

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/zeromicro/go-zero/core/stores/postgres"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

// testConn connects to the local-dev identity_db (`make infra-up migrate-up`)
// and skips the test if it isn't reachable. These are integration tests
// against real Postgres — JSONB, citext, and the partial unique indexes in
// migrations/identity/000002_schema.up.sql don't have a meaningful mock
// stand-in — not unit tests. CI does not run these yet (no Postgres
// service container wired into .github/workflows/ci.yml); that's a
// follow-up, same category as the outbox/idempotency purge jobs noted
// elsewhere as not yet built.
func testConn(t *testing.T) sqlx.SqlConn {
	t.Helper()
	conn := postgres.New("postgres://qrmenu:qrmenu@127.0.0.1:5437/identity_db?sslmode=disable")
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	var one int
	if err := conn.QueryRowCtx(ctx, &one, "SELECT 1"); err != nil {
		t.Skipf("identity_db not reachable (%v) — run `make infra-up migrate-up` to enable these integration tests", err)
	}
	return conn
}

func TestStaffModel_insertFindUpdateDeactivate(t *testing.T) {
	conn := testConn(t)
	m := NewStaffModel(conn)
	ctx := context.Background()
	venueID := uuid.NewString()

	created, err := m.Insert(ctx, venueID, "Alice", "alice-"+venueID+"@example.com", "hash1", "admin")
	if err != nil {
		t.Fatalf("Insert: %v", err)
	}
	t.Cleanup(func() { _, _ = conn.ExecCtx(context.Background(), "DELETE FROM staff WHERE id = $1", created.ID) })

	if created.IsActive != true || created.Role != "admin" {
		t.Fatalf("unexpected created row: %+v", created)
	}

	found, err := m.FindByID(ctx, created.ID)
	if err != nil {
		t.Fatalf("FindByID: %v", err)
	}
	if found.Email != created.Email {
		t.Fatalf("FindByID returned a different row: %+v", found)
	}

	byEmail, err := m.FindByVenueAndEmail(ctx, venueID, created.Email)
	if err != nil {
		t.Fatalf("FindByVenueAndEmail: %v", err)
	}
	if byEmail.ID != created.ID {
		t.Fatalf("FindByVenueAndEmail returned the wrong row: %+v", byEmail)
	}

	updated, err := m.Update(ctx, created.ID, venueID, "Alice B.", created.Email, "manager", true)
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if updated.Name != "Alice B." || updated.Role != "manager" {
		t.Fatalf("Update didn't apply: %+v", updated)
	}

	// Update scoped to the wrong venue must not affect the row.
	if _, err := m.Update(ctx, created.ID, uuid.NewString(), "Mallory", created.Email, "admin", true); err == nil {
		t.Fatal("Update with a mismatched venue_id should not find the row")
	}

	deactivated, err := m.Deactivate(ctx, created.ID, venueID)
	if err != nil {
		t.Fatalf("Deactivate: %v", err)
	}
	if !deactivated {
		t.Fatal("expected Deactivate to report a row was affected")
	}
	after, err := m.FindByID(ctx, created.ID)
	if err != nil {
		t.Fatalf("FindByID after deactivate: %v", err)
	}
	if after.IsActive {
		t.Fatal("expected is_active=false after Deactivate")
	}
}

func TestStaffModel_citextCaseInsensitiveLookup(t *testing.T) {
	conn := testConn(t)
	m := NewStaffModel(conn)
	ctx := context.Background()
	venueID := uuid.NewString()
	email := "Bob-" + venueID + "@Example.com"

	created, err := m.Insert(ctx, venueID, "Bob", email, "hash", "waiter")
	if err != nil {
		t.Fatalf("Insert: %v", err)
	}
	t.Cleanup(func() { _, _ = conn.ExecCtx(context.Background(), "DELETE FROM staff WHERE id = $1", created.ID) })

	found, err := m.FindByVenueAndEmail(ctx, venueID, "BOB-"+venueID+"@EXAMPLE.COM")
	if err != nil {
		t.Fatalf("case-insensitive FindByVenueAndEmail: %v", err)
	}
	if found.ID != created.ID {
		t.Fatalf("expected citext to match regardless of case, got: %+v", found)
	}

	if _, err := m.Insert(ctx, venueID, "Bob2", "bob-"+venueID+"@example.com", "hash2", "cook"); err == nil {
		t.Fatal("expected a case-varied duplicate email in the same venue to violate the unique constraint")
	}
}

func TestStaffModel_listPagination(t *testing.T) {
	conn := testConn(t)
	m := NewStaffModel(conn)
	ctx := context.Background()
	venueID := uuid.NewString()

	var ids []string
	for i := 0; i < 3; i++ {
		s, err := m.Insert(ctx, venueID, "Staff", uuid.NewString()+"@example.com", "hash", "cook")
		if err != nil {
			t.Fatalf("Insert #%d: %v", i, err)
		}
		ids = append(ids, s.ID)
		time.Sleep(2 * time.Millisecond) // ensure distinct created_at for a deterministic order
	}
	t.Cleanup(func() {
		for _, id := range ids {
			_, _ = conn.ExecCtx(context.Background(), "DELETE FROM staff WHERE id = $1", id)
		}
	})

	page1, err := m.List(ctx, venueID, Cursor{}, 2)
	if err != nil {
		t.Fatalf("List page1: %v", err)
	}
	if len(page1) != 2 {
		t.Fatalf("expected 2 rows in page1, got %d", len(page1))
	}

	cursor := Cursor{CreatedAt: page1[len(page1)-1].CreatedAt, ID: page1[len(page1)-1].ID}
	page2, err := m.List(ctx, venueID, cursor, 2)
	if err != nil {
		t.Fatalf("List page2: %v", err)
	}
	if len(page2) != 1 {
		t.Fatalf("expected 1 row in page2, got %d", len(page2))
	}

	seen := map[string]bool{}
	for _, s := range append(page1, page2...) {
		if seen[s.ID] {
			t.Fatalf("row %s returned on more than one page", s.ID)
		}
		seen[s.ID] = true
	}
	for _, id := range ids {
		if !seen[id] {
			t.Fatalf("row %s never returned across pages", id)
		}
	}

	// Round-trip through the wire cursor encoding too.
	encoded := cursor.Encode()
	decoded, err := DecodeCursor(encoded)
	if err != nil {
		t.Fatalf("DecodeCursor: %v", err)
	}
	if !decoded.CreatedAt.Equal(cursor.CreatedAt) || decoded.ID != cursor.ID {
		t.Fatalf("cursor didn't round-trip: got %+v, want %+v", decoded, cursor)
	}
}

func TestRefreshTokenModel_lifecycle(t *testing.T) {
	conn := testConn(t)
	staffModel := NewStaffModel(conn)
	rtModel := NewRefreshTokenModel(conn)
	ctx := context.Background()
	venueID := uuid.NewString()

	staff, err := staffModel.Insert(ctx, venueID, "Carol", uuid.NewString()+"@example.com", "hash", "admin")
	if err != nil {
		t.Fatalf("Insert staff: %v", err)
	}
	t.Cleanup(func() { _, _ = conn.ExecCtx(context.Background(), "DELETE FROM staff WHERE id = $1", staff.ID) })

	rt1, err := rtModel.Insert(ctx, staff.ID, "hash-of-token-1", time.Now().Add(time.Hour))
	if err != nil {
		t.Fatalf("Insert refresh token: %v", err)
	}
	if !rt1.Active() {
		t.Fatalf("freshly inserted token should be Active: %+v", rt1)
	}

	found, err := rtModel.FindByHash(ctx, "hash-of-token-1")
	if err != nil {
		t.Fatalf("FindByHash: %v", err)
	}
	if found.ID != rt1.ID {
		t.Fatalf("FindByHash returned the wrong row: %+v", found)
	}

	rt2, err := rtModel.Insert(ctx, staff.ID, "hash-of-token-2", time.Now().Add(time.Hour))
	if err != nil {
		t.Fatalf("Insert rotated refresh token: %v", err)
	}
	if err := rtModel.MarkReplaced(ctx, rt1.ID, rt2.ID); err != nil {
		t.Fatalf("MarkReplaced: %v", err)
	}
	rt1Reloaded, err := rtModel.FindByHash(ctx, "hash-of-token-1")
	if err != nil {
		t.Fatalf("FindByHash after replace: %v", err)
	}
	if rt1Reloaded.Active() {
		t.Fatal("a replaced token must not be Active")
	}
	if !rt1Reloaded.ReplacedBy.Valid || rt1Reloaded.ReplacedBy.String != rt2.ID {
		t.Fatalf("expected replaced_by to chain to rt2, got: %+v", rt1Reloaded)
	}

	if err := rtModel.RevokeAllForStaff(ctx, staff.ID); err != nil {
		t.Fatalf("RevokeAllForStaff: %v", err)
	}
	rt2Reloaded, err := rtModel.FindByHash(ctx, "hash-of-token-2")
	if err != nil {
		t.Fatalf("FindByHash after RevokeAllForStaff: %v", err)
	}
	if rt2Reloaded.Active() {
		t.Fatal("RevokeAllForStaff should have revoked the still-active rt2 too")
	}
}

func TestSigningKeyModel_ensureActiveIdempotentAndDemotes(t *testing.T) {
	conn := testConn(t)
	m := NewSigningKeyModel(conn)
	ctx := context.Background()
	kidA := "test-kid-a-" + uuid.NewString()
	kidB := "test-kid-b-" + uuid.NewString()
	t.Cleanup(func() {
		_, _ = conn.ExecCtx(context.Background(), "DELETE FROM signing_keys WHERE kid IN ($1, $2)", kidA, kidB)
	})

	if err := m.EnsureActive(ctx, kidA, "RSA", "RS256", []byte(`{"n":"a","e":"AQAB"}`)); err != nil {
		t.Fatalf("EnsureActive(kidA) #1: %v", err)
	}
	// Idempotent: calling again with the same kid must not error or duplicate.
	if err := m.EnsureActive(ctx, kidA, "RSA", "RS256", []byte(`{"n":"a","e":"AQAB"}`)); err != nil {
		t.Fatalf("EnsureActive(kidA) #2 (idempotent replay): %v", err)
	}

	keys, err := m.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	var gotA *SigningKey
	for i := range keys {
		if keys[i].Kid == kidA {
			gotA = &keys[i]
		}
	}
	if gotA == nil || !gotA.IsActive {
		t.Fatalf("expected exactly one active row for kidA, got: %+v", keys)
	}

	if err := m.EnsureActive(ctx, kidB, "RSA", "RS256", []byte(`{"n":"b","e":"AQAB"}`)); err != nil {
		t.Fatalf("EnsureActive(kidB): %v", err)
	}

	keys, err = m.List(ctx)
	if err != nil {
		t.Fatalf("List after rotation: %v", err)
	}
	var gotA2, gotB *SigningKey
	for i := range keys {
		switch keys[i].Kid {
		case kidA:
			gotA2 = &keys[i]
		case kidB:
			gotB = &keys[i]
		}
	}
	if gotA2 == nil || gotA2.IsActive || !gotA2.RetiredAt.Valid {
		t.Fatalf("expected kidA to be retired after kidB became active, got: %+v", gotA2)
	}
	if gotB == nil || !gotB.IsActive {
		t.Fatalf("expected kidB to be active, got: %+v", gotB)
	}
}
