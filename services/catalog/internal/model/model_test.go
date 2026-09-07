package model

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/zeromicro/go-zero/core/stores/postgres"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

// testConn connects to the local-dev catalog_db (`make infra-up migrate-up`)
// and skips the test if it isn't reachable. Integration tests against
// real Postgres — JSONB, text[] arrays, ANY(uuid[]) casts and the
// composite FK in migrations/catalog/000002_schema.up.sql don't have a
// meaningful mock stand-in.
func testConn(t *testing.T) sqlx.SqlConn {
	t.Helper()
	conn := postgres.New("postgres://qrmenu:qrmenu@127.0.0.1:5437/catalog_db?sslmode=disable")
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	var one int
	if err := conn.QueryRowCtx(ctx, &one, "SELECT 1"); err != nil {
		t.Skipf("catalog_db not reachable (%v) — run `make infra-up migrate-up` to enable these integration tests", err)
	}
	return conn
}

// venueFixture inserts a venue directly via SQL (there is no CreateVenue
// RPC — provisioning is a platform-operator action out of scope in R1)
// plus its first QR key, and registers cleanup. Returns the venue id.
func venueFixture(t *testing.T, conn sqlx.SqlConn) string {
	t.Helper()
	ctx := context.Background()
	venueID := uuid.NewString()
	slug := "venue-" + venueID

	_, err := conn.ExecCtx(ctx, `
		INSERT INTO venues (id, slug, name, currency, locales, default_locale, timezone)
		VALUES ($1, $2, 'Test Venue', 'USD', '{en,ru}'::text[], 'en', 'UTC')`,
		venueID, slug)
	if err != nil {
		t.Fatalf("insert venue fixture: %v", err)
	}
	t.Cleanup(func() { _, _ = conn.ExecCtx(context.Background(), "DELETE FROM venues WHERE id = $1", venueID) })

	if _, err := NewVenueQRKeyModel(conn).Insert(ctx, venueID, 1, []byte("test-secret-material-32-bytes!!")); err != nil {
		t.Fatalf("insert venue qr key fixture: %v", err)
	}
	return venueID
}

func hallFixture(t *testing.T, conn sqlx.SqlConn, venueID string) string {
	t.Helper()
	h, err := NewHallModel(conn).Insert(context.Background(), venueID, "Main Hall", 0)
	if err != nil {
		t.Fatalf("insert hall fixture: %v", err)
	}
	return h.ID
}

func categoryFixture(t *testing.T, conn sqlx.SqlConn, venueID string) string {
	t.Helper()
	c, err := NewCategoryModel(conn).Insert(context.Background(), venueID, localizedText{"en": "Drinks"}, 0, true, "")
	if err != nil {
		t.Fatalf("insert category fixture: %v", err)
	}
	return c.ID
}

// ---------------------------------------------------------------------
// Venue
// ---------------------------------------------------------------------

func TestVenueModel_findAndUpdateSettings(t *testing.T) {
	conn := testConn(t)
	venueID := venueFixture(t, conn)
	m := NewVenueModel(conn)
	ctx := context.Background()

	found, err := m.FindByID(ctx, venueID)
	if err != nil {
		t.Fatalf("FindByID: %v", err)
	}
	if len(found.Locales) != 2 || found.Locales[0] != "en" || found.Locales[1] != "ru" {
		t.Fatalf("locales didn't round-trip through the text[] scanner: %+v", found.Locales)
	}

	bySlug, err := m.FindBySlug(ctx, found.Slug)
	if err != nil {
		t.Fatalf("FindBySlug: %v", err)
	}
	if bySlug.ID != venueID {
		t.Fatalf("FindBySlug returned the wrong venue: %+v", bySlug)
	}

	found.Name = "Renamed Venue"
	found.Locales = stringSlice{"en", "de", "fr"}
	found.ServiceChargeBps = 1000
	updated, err := m.UpdateSettings(ctx, found)
	if err != nil {
		t.Fatalf("UpdateSettings: %v", err)
	}
	if updated.Name != "Renamed Venue" || updated.ServiceChargeBps != 1000 {
		t.Fatalf("UpdateSettings didn't apply: %+v", updated)
	}
	if len(updated.Locales) != 3 || updated.Locales[2] != "fr" {
		t.Fatalf("updated locales didn't round-trip: %+v", updated.Locales)
	}
}

func TestVenueModel_bumpMenuVersion(t *testing.T) {
	conn := testConn(t)
	venueID := venueFixture(t, conn)
	m := NewVenueModel(conn)
	ctx := context.Background()

	before, err := m.FindByID(ctx, venueID)
	if err != nil {
		t.Fatal(err)
	}

	next, err := m.BumpMenuVersion(ctx, venueID)
	if err != nil {
		t.Fatalf("BumpMenuVersion: %v", err)
	}
	if next != before.MenuVersion+1 {
		t.Fatalf("expected menu_version to advance by 1, got %d -> %d", before.MenuVersion, next)
	}
}

// ---------------------------------------------------------------------
// VenueQRKey
// ---------------------------------------------------------------------

func TestVenueQRKeyModel_rotationLifecycle(t *testing.T) {
	conn := testConn(t)
	venueID := venueFixture(t, conn) // already has key_version 1
	m := NewVenueQRKeyModel(conn)
	ctx := context.Background()

	current, err := m.FindCurrent(ctx, venueID)
	if err != nil {
		t.Fatalf("FindCurrent: %v", err)
	}
	if current.KeyVersion != 1 || current.ExpiresAt.Valid {
		t.Fatalf("expected key_version 1 to be current: %+v", current)
	}

	// Rotate: expire v1 BEFORE inserting v2 — venue_qr_keys_current_idx
	// (partial unique on venue_id WHERE expires_at IS NULL) allows only
	// one current key per venue, so doing this the other way round
	// briefly has two current rows and fails the constraint (caught by
	// this test before it was reordered).
	graceExpiry := time.Now().Add(30 * 24 * time.Hour)
	if err := m.ExpireAt(ctx, venueID, 1, graceExpiry); err != nil {
		t.Fatalf("ExpireAt v1: %v", err)
	}
	if _, err := m.Insert(ctx, venueID, 2, []byte("second-secret-material-32-bytes!")); err != nil {
		t.Fatalf("Insert v2: %v", err)
	}

	newCurrent, err := m.FindCurrent(ctx, venueID)
	if err != nil {
		t.Fatalf("FindCurrent after rotation: %v", err)
	}
	if newCurrent.KeyVersion != 2 {
		t.Fatalf("expected v2 to be current after rotation, got %+v", newCurrent)
	}

	v1, err := m.FindByVersion(ctx, venueID, 1)
	if err != nil {
		t.Fatalf("FindByVersion(1): %v", err)
	}
	if !v1.ExpiresAt.Valid {
		t.Fatal("expected v1 to now have an expiry")
	}
}

// ---------------------------------------------------------------------
// Hall / Table
// ---------------------------------------------------------------------

func TestHallModel_lifecycle(t *testing.T) {
	conn := testConn(t)
	venueID := venueFixture(t, conn)
	m := NewHallModel(conn)
	ctx := context.Background()

	h, err := m.Insert(ctx, venueID, "Terrace", 1)
	if err != nil {
		t.Fatalf("Insert: %v", err)
	}
	list, err := m.List(ctx, venueID)
	if err != nil || len(list) != 1 {
		t.Fatalf("List: %v (%d rows)", err, len(list))
	}

	updated, err := m.Update(ctx, h.ID, venueID, "Terrace Renamed", 2, true)
	if err != nil || updated.Name != "Terrace Renamed" {
		t.Fatalf("Update: %v (%+v)", err, updated)
	}

	ok, err := m.Deactivate(ctx, h.ID, venueID)
	if err != nil || !ok {
		t.Fatalf("Deactivate: %v (%v)", err, ok)
	}
	found, err := m.FindByID(ctx, venueID, h.ID)
	if err != nil || found.IsActive {
		t.Fatalf("expected is_active=false after Deactivate: %v (%+v)", err, found)
	}
}

func TestTableModel_insertRequiresValidKeyVersion(t *testing.T) {
	conn := testConn(t)
	venueID := venueFixture(t, conn)
	hallID := hallFixture(t, conn, venueID)
	m := NewTableModel(conn)
	ctx := context.Background()

	if _, err := m.Insert(ctx, venueID, hallID, "T1", 4, "abcdef1234", 999); err == nil {
		t.Fatal("expected the composite FK to reject an unknown key_version")
	}

	tbl, err := m.Insert(ctx, venueID, hallID, "T1", 4, "abcdef1234", 1)
	if err != nil {
		t.Fatalf("Insert with valid key_version: %v", err)
	}

	byCode, err := m.FindByVenueAndCode(ctx, venueID, "abcdef1234")
	if err != nil || byCode.ID != tbl.ID {
		t.Fatalf("FindByVenueAndCode: %v (%+v)", err, byCode)
	}

	ok, err := m.Deactivate(ctx, tbl.ID, venueID)
	if err != nil || !ok {
		t.Fatalf("Deactivate: %v (%v)", err, ok)
	}
}

func TestTableModel_listPaginationAndHallFilter(t *testing.T) {
	conn := testConn(t)
	venueID := venueFixture(t, conn)
	hallA := hallFixture(t, conn, venueID)
	hallB := hallFixture(t, conn, venueID)
	m := NewTableModel(conn)
	ctx := context.Background()

	for i, hall := range []string{hallA, hallA, hallB} {
		code := uuid.NewString()[:12]
		if _, err := m.Insert(ctx, venueID, hall, "T", 4, code, 1); err != nil {
			t.Fatalf("Insert #%d: %v", i, err)
		}
		time.Sleep(2 * time.Millisecond)
	}

	all, err := m.List(ctx, venueID, "", Cursor{}, 10)
	if err != nil || len(all) != 3 {
		t.Fatalf("List all: %v (%d rows)", err, len(all))
	}

	onlyA, err := m.List(ctx, venueID, hallA, Cursor{}, 10)
	if err != nil || len(onlyA) != 2 {
		t.Fatalf("List filtered by hallA: %v (%d rows)", err, len(onlyA))
	}

	page1, err := m.List(ctx, venueID, "", Cursor{}, 2)
	if err != nil || len(page1) != 2 {
		t.Fatalf("List page1: %v (%d rows)", err, len(page1))
	}
	cursor := Cursor{CreatedAt: page1[1].CreatedAt, ID: page1[1].ID}
	page2, err := m.List(ctx, venueID, "", cursor, 2)
	if err != nil || len(page2) != 1 {
		t.Fatalf("List page2: %v (%d rows)", err, len(page2))
	}
}

// ---------------------------------------------------------------------
// Category / MenuItem / AvailabilityWindow
// ---------------------------------------------------------------------

func TestCategoryModel_lifecycleAndDeleteRestrict(t *testing.T) {
	conn := testConn(t)
	venueID := venueFixture(t, conn)
	m := NewCategoryModel(conn)
	itemModel := NewMenuItemModel(conn)
	ctx := context.Background()

	visible, err := m.Insert(ctx, venueID, localizedText{"en": "Mains"}, 0, true, "")
	if err != nil {
		t.Fatalf("Insert visible: %v", err)
	}
	hidden, err := m.Insert(ctx, venueID, localizedText{"en": "Hidden"}, 1, false, "")
	if err != nil {
		t.Fatalf("Insert hidden: %v", err)
	}

	all, err := m.List(ctx, venueID)
	if err != nil || len(all) != 2 {
		t.Fatalf("List: %v (%d rows)", err, len(all))
	}
	visibleOnly, err := m.ListVisible(ctx, venueID)
	if err != nil || len(visibleOnly) != 1 || visibleOnly[0].ID != visible.ID {
		t.Fatalf("ListVisible: %v (%+v)", err, visibleOnly)
	}

	if _, err := m.Update(ctx, hidden.ID, venueID, localizedText{"en": "Now Visible"}, 1, true, ""); err != nil {
		t.Fatalf("Update: %v", err)
	}

	// Deleting a category that still has a menu item must fail the FK.
	if _, err := itemModel.Insert(ctx, venueID, visible.ID, localizedText{"en": "Steak"}, localizedText{}, 2500, "", nil, 0); err != nil {
		t.Fatalf("insert item fixture: %v", err)
	}
	if ok, err := m.Delete(ctx, visible.ID, venueID); err == nil {
		t.Fatalf("expected deleting a non-empty category to fail the FK, got ok=%v", ok)
	}

	// An empty category deletes cleanly.
	ok, err := m.Delete(ctx, hidden.ID, venueID)
	if err != nil || !ok {
		t.Fatalf("Delete empty category: %v (%v)", err, ok)
	}
}

func TestMenuItemModel_lifecycleAndBatchLookup(t *testing.T) {
	conn := testConn(t)
	venueID := venueFixture(t, conn)
	categoryID := categoryFixture(t, conn, venueID)
	m := NewMenuItemModel(conn)
	ctx := context.Background()

	item, err := m.Insert(ctx, venueID, categoryID,
		localizedText{"en": "Latte", "ru": "Латте"}, localizedText{"en": "Espresso with milk"},
		450, "https://example.com/latte.jpg", []string{"milk"}, 0)
	if err != nil {
		t.Fatalf("Insert: %v", err)
	}
	if item.Name["ru"] != "Латте" || len(item.Allergens) != 1 || item.Allergens[0] != "milk" {
		t.Fatalf("unexpected inserted item: %+v", item)
	}

	byIDs, err := m.FindByIDs(ctx, venueID, []string{item.ID, uuid.NewString()})
	if err != nil {
		t.Fatalf("FindByIDs: %v", err)
	}
	if len(byIDs) != 1 || byIDs[0].ID != item.ID {
		t.Fatalf("FindByIDs: expected exactly the one real id back, got %+v", byIDs)
	}

	forMenu, err := m.ListForMenu(ctx, venueID)
	if err != nil || len(forMenu) != 1 {
		t.Fatalf("ListForMenu (active+available): %v (%d rows)", err, len(forMenu))
	}

	if _, err := m.SetAvailability(ctx, item.ID, venueID, false); err != nil {
		t.Fatalf("SetAvailability: %v", err)
	}
	forMenu, err = m.ListForMenu(ctx, venueID)
	if err != nil || len(forMenu) != 0 {
		t.Fatalf("expected ListForMenu to exclude an unavailable item: %v (%d rows)", err, len(forMenu))
	}

	if ok, err := m.Deactivate(ctx, item.ID, venueID); err != nil || !ok {
		t.Fatalf("Deactivate: %v (%v)", err, ok)
	}
}

func TestAvailabilityWindowModel_replaceIsIdempotentAndBatches(t *testing.T) {
	conn := testConn(t)
	venueID := venueFixture(t, conn)
	categoryID := categoryFixture(t, conn, venueID)
	itemModel := NewMenuItemModel(conn)
	windowModel := NewAvailabilityWindowModel(conn)
	ctx := context.Background()

	item, err := itemModel.Insert(ctx, venueID, categoryID, localizedText{"en": "Breakfast Set"}, localizedText{}, 1200, "", nil, 0)
	if err != nil {
		t.Fatal(err)
	}

	if err := windowModel.ReplaceForItem(ctx, item.ID, []AvailabilityWindow{
		{ItemID: item.ID, StartMinuteOfDay: 480, EndMinuteOfDay: 660},
	}); err != nil {
		t.Fatalf("ReplaceForItem #1: %v", err)
	}
	// Replacing again with a different set must not accumulate rows.
	if err := windowModel.ReplaceForItem(ctx, item.ID, []AvailabilityWindow{
		{ItemID: item.ID, StartMinuteOfDay: 0, EndMinuteOfDay: 1439},
	}); err != nil {
		t.Fatalf("ReplaceForItem #2: %v", err)
	}

	windows, err := windowModel.ListByItemIDs(ctx, []string{item.ID})
	if err != nil {
		t.Fatalf("ListByItemIDs: %v", err)
	}
	if len(windows) != 1 || windows[0].StartMinuteOfDay != 0 || windows[0].EndMinuteOfDay != 1439 {
		t.Fatalf("expected exactly the second window set, got %+v", windows)
	}
}

// ---------------------------------------------------------------------
// ModifierGroup / ModifierOption
// ---------------------------------------------------------------------

func TestModifierModel_lifecycleAndCascadeDelete(t *testing.T) {
	conn := testConn(t)
	venueID := venueFixture(t, conn)
	categoryID := categoryFixture(t, conn, venueID)
	itemModel := NewMenuItemModel(conn)
	groupModel := NewModifierGroupModel(conn)
	optionModel := NewModifierOptionModel(conn)
	ctx := context.Background()

	item, err := itemModel.Insert(ctx, venueID, categoryID, localizedText{"en": "Coffee"}, localizedText{}, 350, "", nil, 0)
	if err != nil {
		t.Fatal(err)
	}

	group, err := groupModel.Insert(ctx, item.ID, localizedText{"en": "Milk"}, 0, 1, false)
	if err != nil {
		t.Fatalf("Insert group: %v", err)
	}

	options, err := optionModel.ReplaceForGroup(ctx, group.ID, []ModifierOption{
		{Name: localizedText{"en": "Oat milk"}, PriceDeltaMinor: 50, SortOrder: 0},
		{Name: localizedText{"en": "Whole milk"}, PriceDeltaMinor: 0, SortOrder: 1},
	})
	if err != nil || len(options) != 2 {
		t.Fatalf("ReplaceForGroup: %v (%d rows)", err, len(options))
	}

	groups, err := groupModel.ListByItemIDs(ctx, []string{item.ID})
	if err != nil || len(groups) != 1 {
		t.Fatalf("ListByItemIDs: %v (%d rows)", err, len(groups))
	}
	byGroup, err := optionModel.ListByGroupIDs(ctx, []string{group.ID})
	if err != nil || len(byGroup) != 2 {
		t.Fatalf("ListByGroupIDs: %v (%d rows)", err, len(byGroup))
	}
	byIDs, err := optionModel.FindByIDs(ctx, []string{options[0].ID})
	if err != nil || len(byIDs) != 1 {
		t.Fatalf("FindByIDs: %v (%d rows)", err, len(byIDs))
	}

	ok, err := groupModel.Delete(ctx, group.ID, item.ID)
	if err != nil || !ok {
		t.Fatalf("Delete group: %v (%v)", err, ok)
	}
	remaining, err := optionModel.ListByGroupIDs(ctx, []string{group.ID})
	if err != nil || len(remaining) != 0 {
		t.Fatalf("expected options to cascade-delete with their group: %v (%d rows)", err, len(remaining))
	}
}

// ---------------------------------------------------------------------
// Outbox
// ---------------------------------------------------------------------

func TestOutboxModel_insert(t *testing.T) {
	conn := testConn(t)
	venueID := venueFixture(t, conn)
	m := NewOutboxModel(conn)
	ctx := context.Background()

	err := m.Insert(ctx, "catalog.item_availability_changed", venueID, "qrmenu.catalog.v1", "item-1",
		map[string]any{"item_id": "item-1", "is_available": false}, "trace-1")
	if err != nil {
		t.Fatalf("Insert: %v", err)
	}

	var count int
	if err := conn.QueryRowCtx(ctx, &count, "SELECT count(*) FROM outbox WHERE venue_id = $1 AND sent_at IS NULL", venueID); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("expected exactly 1 unsent outbox row, got %d", count)
	}
}

// TestLocalizedTextUpdatesMergeRatherThanReplace pins the semantics
// documented on localizedText: an update naming one locale must leave the
// others intact.
//
// This is the behaviour the admin API depends on — §8.1's write bodies
// edit a single locale at a time, so a replacing update would make
// "rename this in English" silently delete every translation. Worth a test
// of its own because it is invisible in the Go signature: the merge lives
// in the SQL.
func TestLocalizedTextUpdatesMergeRatherThanReplace(t *testing.T) {
	conn := testConn(t)
	venueID := venueFixture(t, conn)
	ctx := context.Background()

	t.Run("category", func(t *testing.T) {
		m := NewCategoryModel(conn)
		created, err := m.Insert(ctx, venueID, localizedText{"en": "Drinks", "ru": "Напитки"}, 0, true, "")
		if err != nil {
			t.Fatalf("insert: %v", err)
		}

		updated, err := m.Update(ctx, created.ID, venueID, localizedText{"en": "Beverages"}, 1, true, "")
		if err != nil {
			t.Fatalf("update: %v", err)
		}
		if updated.Name["en"] != "Beverages" {
			t.Errorf("en = %q, want the new value", updated.Name["en"])
		}
		if updated.Name["ru"] != "Напитки" {
			t.Errorf("ru = %q, want the untouched translation to survive", updated.Name["ru"])
		}
		if updated.SortOrder != 1 {
			t.Errorf("non-localized fields should still be replaced outright: sortOrder = %d", updated.SortOrder)
		}
	})

	t.Run("menu item name and description", func(t *testing.T) {
		catModel := NewCategoryModel(conn)
		cat, err := catModel.Insert(ctx, venueID, localizedText{"en": "Food"}, 0, true, "")
		if err != nil {
			t.Fatalf("insert category: %v", err)
		}
		m := NewMenuItemModel(conn)
		created, err := m.Insert(ctx, venueID, cat.ID,
			localizedText{"en": "Soup", "ru": "Суп"},
			localizedText{"en": "Hot", "ru": "Горячий"},
			500, "", nil, 0)
		if err != nil {
			t.Fatalf("insert item: %v", err)
		}

		updated, err := m.Update(ctx, created.ID, venueID, cat.ID,
			localizedText{"en": "Broth"}, localizedText{"en": "Very hot"},
			600, "", nil, 0)
		if err != nil {
			t.Fatalf("update: %v", err)
		}
		if updated.Name["en"] != "Broth" || updated.Name["ru"] != "Суп" {
			t.Errorf("name = %v, want en updated and ru preserved", updated.Name)
		}
		if updated.Description["en"] != "Very hot" || updated.Description["ru"] != "Горячий" {
			t.Errorf("description = %v, want en updated and ru preserved", updated.Description)
		}
		if updated.BasePriceMinor != 600 {
			t.Errorf("price = %d, want 600", updated.BasePriceMinor)
		}
	})

	t.Run("modifier group", func(t *testing.T) {
		catModel := NewCategoryModel(conn)
		cat, err := catModel.Insert(ctx, venueID, localizedText{"en": "Extras"}, 0, true, "")
		if err != nil {
			t.Fatalf("insert category: %v", err)
		}
		item, err := NewMenuItemModel(conn).Insert(ctx, venueID, cat.ID,
			localizedText{"en": "Coffee"}, localizedText{"en": ""}, 300, "", nil, 0)
		if err != nil {
			t.Fatalf("insert item: %v", err)
		}
		m := NewModifierGroupModel(conn)
		created, err := m.Insert(ctx, item.ID, localizedText{"en": "Milk", "ru": "Молоко"}, 0, 1, false)
		if err != nil {
			t.Fatalf("insert group: %v", err)
		}

		updated, err := m.Update(ctx, created.ID, item.ID, localizedText{"en": "Milk choice"}, 1, 2, true)
		if err != nil {
			t.Fatalf("update: %v", err)
		}
		if updated.Name["en"] != "Milk choice" || updated.Name["ru"] != "Молоко" {
			t.Errorf("name = %v, want en updated and ru preserved", updated.Name)
		}
		if !updated.Required || updated.MaxSelect != 2 {
			t.Errorf("scalar fields not applied: %+v", updated)
		}
	})
}
