package model

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/zeromicro/go-zero/core/stores/postgres"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

// testConn connects to the local-dev order_db (`make infra-up migrate-up`)
// and skips the test if it isn't reachable.
//
// These are integration tests against real Postgres because that is where
// the behaviour under test actually lives: partial unique indexes as
// concurrency control, ON CONFLICT arbiters, compare-and-set UPDATEs,
// FOR UPDATE row locks and the atomic order-number counter have no
// meaningful mock stand-in — a fake would only re-assert what the fake
// was written to do.
func testConn(t *testing.T) sqlx.SqlConn {
	t.Helper()
	conn := postgres.New("postgres://qrmenu:qrmenu@127.0.0.1:5437/order_db?sslmode=disable")
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	var one int
	if err := conn.QueryRowCtx(ctx, &one, "SELECT 1"); err != nil {
		t.Skipf("order_db not reachable (%v) — run `make infra-up migrate-up` to enable these integration tests", err)
	}
	return conn
}

// order_db holds no cross-service foreign keys (docs/TZ.md §6), so a
// venue or table "fixture" is just an id nothing else has to know about.
func newID() string { return uuid.NewString() }

// openSession opens a table session and registers cleanup of everything
// that will hang off it.
func openSession(t *testing.T, conn sqlx.SqlConn, venueID, tableID string) *TableSession {
	t.Helper()
	s, err := NewTableSessionModel(conn).OpenOrJoin(context.Background(), venueID, tableID, "USD")
	if err != nil {
		t.Fatalf("open table session: %v", err)
	}
	t.Cleanup(func() {
		ctx := context.Background()
		_, _ = conn.ExecCtx(ctx, `DELETE FROM outbox WHERE venue_id = $1`, venueID)
		_, _ = conn.ExecCtx(ctx, `DELETE FROM service_requests WHERE table_session_id = $1`, s.ID)
		_, _ = conn.ExecCtx(ctx, `DELETE FROM orders WHERE table_session_id = $1`, s.ID)
		_, _ = conn.ExecCtx(ctx, `DELETE FROM guest_sessions WHERE table_session_id = $1`, s.ID)
		_, _ = conn.ExecCtx(ctx, `DELETE FROM table_sessions WHERE id = $1`, s.ID)
		_, _ = conn.ExecCtx(ctx, `DELETE FROM order_number_counters WHERE venue_id = $1`, venueID)
		_, _ = conn.ExecCtx(ctx, `DELETE FROM idempotency_keys WHERE venue_id = $1`, venueID)
	})
	return s
}

func registerGuest(t *testing.T, conn sqlx.SqlConn, session *TableSession) string {
	t.Helper()
	id := newID()
	if _, err := NewGuestSessionModel(conn).EnsureExists(
		context.Background(), id, session.VenueID, session.TableID, session.ID, 4*time.Hour); err != nil {
		t.Fatalf("register guest session: %v", err)
	}
	return id
}

func today() time.Time {
	n := time.Now().UTC()
	return time.Date(n.Year(), n.Month(), n.Day(), 0, 0, 0, 0, time.UTC)
}

// ---------------------------------------------------------------------
// Table sessions (FR-O9, FR-S6)
// ---------------------------------------------------------------------

// TestTableSessionModel_openOrJoin is FR-O9: the first order at a free
// table opens a session, later orders join it. The guarantee comes from
// table_sessions_one_open_per_table_idx, so this exercises the real
// index rather than the Go around it.
func TestTableSessionModel_openOrJoin(t *testing.T) {
	conn := testConn(t)
	venueID, tableID := newID(), newID()
	sessions := NewTableSessionModel(conn)

	first := openSession(t, conn, venueID, tableID)
	if first.Status != TableSessionOpen {
		t.Errorf("status = %q, want open", first.Status)
	}
	if first.TotalMinor != 0 {
		t.Errorf("a new session should start at zero, got %d", first.TotalMinor)
	}

	second, err := sessions.OpenOrJoin(context.Background(), venueID, tableID, "USD")
	if err != nil {
		t.Fatalf("second OpenOrJoin: %v", err)
	}
	if second.ID != first.ID {
		t.Errorf("a second guest opened a new session (%s) instead of joining %s", second.ID, first.ID)
	}

	// A different table is genuinely a different session.
	otherTable := newID()
	other := openSession(t, conn, venueID, otherTable)
	if other.ID == first.ID {
		t.Error("two tables shared one session")
	}
}

func TestTableSessionModel_closeThenReopenIsANewSession(t *testing.T) {
	conn := testConn(t)
	ctx := context.Background()
	venueID, tableID := newID(), newID()
	sessions := NewTableSessionModel(conn)

	first := openSession(t, conn, venueID, tableID)
	staffID := newID()

	closed, err := sessions.Close(ctx, venueID, first.ID, "cash", staffID)
	if err != nil {
		t.Fatalf("close: %v", err)
	}
	if closed.Status != TableSessionClosed {
		t.Errorf("status = %q, want closed", closed.Status)
	}
	if !closed.ClosedAt.Valid {
		t.Error("closed_at was not stamped")
	}
	if closed.PaymentMethod.String != "cash" {
		t.Errorf("payment_method = %q, want cash", closed.PaymentMethod.String)
	}
	if closed.ClosedByStaffID.String != staffID {
		t.Errorf("closed_by_staff_id = %q, want %q", closed.ClosedByStaffID.String, staffID)
	}

	// The next party at the same table gets a fresh session — the partial
	// index only constrains *open* rows.
	next, err := sessions.OpenOrJoin(ctx, venueID, tableID, "USD")
	if err != nil {
		t.Fatalf("reopen after close: %v", err)
	}
	t.Cleanup(func() { _, _ = conn.ExecCtx(ctx, `DELETE FROM table_sessions WHERE id = $1`, next.ID) })
	if next.ID == first.ID {
		t.Error("reopening returned the closed session")
	}
}

// TestTableSessionModel_closeIsConditional covers the concurrency
// property CloseTableSession relies on: two staff settling the same table
// produce one close and one honest failure, not a lost update.
func TestTableSessionModel_closeIsConditional(t *testing.T) {
	conn := testConn(t)
	ctx := context.Background()
	venueID, tableID := newID(), newID()
	sessions := NewTableSessionModel(conn)

	s := openSession(t, conn, venueID, tableID)
	if _, err := sessions.Close(ctx, venueID, s.ID, "card_terminal", newID()); err != nil {
		t.Fatalf("first close: %v", err)
	}

	_, err := sessions.Close(ctx, venueID, s.ID, "cash", newID())
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("second close error = %v, want ErrNotFound", err)
	}

	// The first close's payment method must survive the second attempt.
	after, err := sessions.FindByID(ctx, venueID, s.ID)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if after.PaymentMethod.String != "card_terminal" {
		t.Errorf("payment_method = %q, want the first close's card_terminal", after.PaymentMethod.String)
	}
}

func TestTableSessionModel_totalsAndActiveListing(t *testing.T) {
	conn := testConn(t)
	ctx := context.Background()
	venueID := newID()
	sessions := NewTableSessionModel(conn)

	a := openSession(t, conn, venueID, newID())
	b := openSession(t, conn, venueID, newID())

	if err := sessions.AddToTotal(ctx, a.ID, 2500); err != nil {
		t.Fatalf("add to total: %v", err)
	}
	if err := sessions.AddToTotal(ctx, a.ID, 1500); err != nil {
		t.Fatalf("add to total: %v", err)
	}
	// A cancellation subtracts.
	if err := sessions.AddToTotal(ctx, a.ID, -1000); err != nil {
		t.Fatalf("subtract from total: %v", err)
	}

	reloaded, err := sessions.FindByID(ctx, venueID, a.ID)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if reloaded.TotalMinor != 3000 {
		t.Errorf("total = %d, want 3000", reloaded.TotalMinor)
	}

	active, err := sessions.ListActive(ctx, venueID)
	if err != nil {
		t.Fatalf("list active: %v", err)
	}
	if len(active) != 2 {
		t.Fatalf("listed %d active sessions, want 2", len(active))
	}

	// Closing one drops it from the floor view.
	if _, err := sessions.Close(ctx, venueID, b.ID, "other", newID()); err != nil {
		t.Fatalf("close: %v", err)
	}
	active, err = sessions.ListActive(ctx, venueID)
	if err != nil {
		t.Fatalf("list active after close: %v", err)
	}
	if len(active) != 1 || active[0].ID != a.ID {
		t.Errorf("active listing = %v, want just %s", active, a.ID)
	}
}

// TestTableSessionModel_totalCannotGoNegative documents that the schema's
// CHECK is the backstop: a bug that over-subtracts fails loudly instead of
// showing a guest a negative bill.
func TestTableSessionModel_totalCannotGoNegative(t *testing.T) {
	conn := testConn(t)
	venueID, tableID := newID(), newID()
	s := openSession(t, conn, venueID, tableID)

	if err := NewTableSessionModel(conn).AddToTotal(context.Background(), s.ID, -1); err == nil {
		t.Error("subtracting below zero was accepted; the CHECK constraint is missing")
	}
}

// ---------------------------------------------------------------------
// Guest sessions (FR-O1)
// ---------------------------------------------------------------------

func TestGuestSessionModel_ensureExistsIsIdempotent(t *testing.T) {
	conn := testConn(t)
	ctx := context.Background()
	venueID, tableID := newID(), newID()
	session := openSession(t, conn, venueID, tableID)
	guests := NewGuestSessionModel(conn)

	id := newID()
	first, err := guests.EnsureExists(ctx, id, venueID, tableID, session.ID, 4*time.Hour)
	if err != nil {
		t.Fatalf("first EnsureExists: %v", err)
	}
	// The id must be the caller's, never generated — it has to match the
	// guest_session_id already inside the guest's JWT.
	if first.ID != id {
		t.Fatalf("id = %s, want the caller's %s", first.ID, id)
	}
	if first.ExpiresAt.Before(time.Now().Add(3 * time.Hour)) {
		t.Errorf("expires_at = %s, want roughly 4h out", first.ExpiresAt)
	}

	time.Sleep(10 * time.Millisecond)
	second, err := guests.EnsureExists(ctx, id, venueID, tableID, session.ID, 4*time.Hour)
	if err != nil {
		t.Fatalf("second EnsureExists: %v", err)
	}
	if !second.LastSeenAt.After(first.LastSeenAt) {
		t.Errorf("last_seen_at did not advance: %s -> %s", first.LastSeenAt, second.LastSeenAt)
	}
}

// TestGuestSessionModel_ensureExistsNeverRepointsSession is the property
// CreateOrder's SessionClosed check depends on: a token from an earlier
// session at the same table must not silently migrate onto the new
// party's bill.
func TestGuestSessionModel_ensureExistsNeverRepointsSession(t *testing.T) {
	conn := testConn(t)
	ctx := context.Background()
	venueID, tableID := newID(), newID()
	sessions := NewTableSessionModel(conn)
	guests := NewGuestSessionModel(conn)

	first := openSession(t, conn, venueID, tableID)
	guestID := registerGuest(t, conn, first)

	if _, err := sessions.Close(ctx, venueID, first.ID, "cash", newID()); err != nil {
		t.Fatalf("close: %v", err)
	}
	next, err := sessions.OpenOrJoin(ctx, venueID, tableID, "USD")
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	t.Cleanup(func() {
		_, _ = conn.ExecCtx(ctx, `DELETE FROM guest_sessions WHERE table_session_id = $1`, next.ID)
		_, _ = conn.ExecCtx(ctx, `DELETE FROM table_sessions WHERE id = $1`, next.ID)
	})

	// The old guest comes back with their old token.
	rejoined, err := guests.EnsureExists(ctx, guestID, venueID, tableID, next.ID, 4*time.Hour)
	if err != nil {
		t.Fatalf("EnsureExists for a returning guest: %v", err)
	}
	if rejoined.TableSessionID != first.ID {
		t.Errorf("table_session_id = %s, want it pinned to the original %s", rejoined.TableSessionID, first.ID)
	}
}

func TestGuestSessionModel_revokeForTableSession(t *testing.T) {
	conn := testConn(t)
	ctx := context.Background()
	venueID, tableID := newID(), newID()
	session := openSession(t, conn, venueID, tableID)
	guests := NewGuestSessionModel(conn)

	a := registerGuest(t, conn, session)
	b := registerGuest(t, conn, session)

	if err := guests.RevokeForTableSession(ctx, session.ID); err != nil {
		t.Fatalf("revoke: %v", err)
	}
	for _, id := range []string{a, b} {
		g, err := guests.FindByID(ctx, id)
		if err != nil {
			t.Fatalf("find guest %s: %v", id, err)
		}
		if !g.RevokedAt.Valid {
			t.Errorf("guest %s was not revoked", id)
		}
	}
}

// ---------------------------------------------------------------------
// Order numbering (FR-O8)
// ---------------------------------------------------------------------

// TestOrderModel_numberSequenceIsAtomic exercises the upsert-returning
// counter. Sequential here, but the point is that each call is a single
// statement: no SELECT-then-UPDATE window exists for two orders to read
// the same value.
func TestOrderModel_numberSequenceIsAtomic(t *testing.T) {
	conn := testConn(t)
	ctx := context.Background()
	venueID := newID()
	orders := NewOrderModel(conn)
	date := today()
	t.Cleanup(func() {
		_, _ = conn.ExecCtx(ctx, `DELETE FROM order_number_counters WHERE venue_id = $1`, venueID)
	})

	for want := 1; want <= 5; want++ {
		got, err := orders.NewOrderNumberSeq(ctx, venueID, date)
		if err != nil {
			t.Fatalf("seq %d: %v", want, err)
		}
		if got != want {
			t.Fatalf("sequence returned %d, want %d", got, want)
		}
	}

	// A different business day restarts at 1 — that is what makes the
	// number human-sized.
	tomorrow := date.AddDate(0, 0, 1)
	got, err := orders.NewOrderNumberSeq(ctx, venueID, tomorrow)
	if err != nil {
		t.Fatalf("next day: %v", err)
	}
	if got != 1 {
		t.Errorf("a new business day started at %d, want 1", got)
	}

	// So does a different venue on the same day.
	otherVenue := newID()
	t.Cleanup(func() {
		_, _ = conn.ExecCtx(ctx, `DELETE FROM order_number_counters WHERE venue_id = $1`, otherVenue)
	})
	got, err = orders.NewOrderNumberSeq(ctx, otherVenue, date)
	if err != nil {
		t.Fatalf("other venue: %v", err)
	}
	if got != 1 {
		t.Errorf("a different venue started at %d, want 1", got)
	}
}

func TestOrderModel_numberIsUniquePerVenueDay(t *testing.T) {
	conn := testConn(t)
	ctx := context.Background()
	venueID, tableID := newID(), newID()
	session := openSession(t, conn, venueID, tableID)
	guestID := registerGuest(t, conn, session)
	orders := NewOrderModel(conn)

	params := func(number string) InsertOrderParams {
		return InsertOrderParams{
			VenueID: venueID, TableID: tableID, TableSessionID: session.ID, GuestSessionID: guestID,
			BusinessDate: today(), Number: number, TotalMinor: 100, Currency: "USD", MenuVersion: "1",
			Items: []InsertOrderItemParams{{
				MenuItemID: newID(), Name: "Latte", UnitPriceMinor: 100, Qty: 1, LineTotalMinor: 100,
			}},
		}
	}

	if _, err := orders.Insert(ctx, params("A-001")); err != nil {
		t.Fatalf("first insert: %v", err)
	}
	_, err := orders.Insert(ctx, params("A-001"))
	if err == nil {
		t.Fatal("a duplicate number on the same venue-day was accepted")
	}
	if !IsUniqueViolation(err) {
		t.Errorf("error = %v, want a unique violation", err)
	}
}

// ---------------------------------------------------------------------
// Orders
// ---------------------------------------------------------------------

func seedOrder(t *testing.T, conn sqlx.SqlConn, session *TableSession, guestID, number string, items []InsertOrderItemParams) *Order {
	t.Helper()
	total := int64(0)
	for _, it := range items {
		total += it.LineTotalMinor
	}
	o, err := NewOrderModel(conn).Insert(context.Background(), InsertOrderParams{
		VenueID: session.VenueID, TableID: session.TableID, TableSessionID: session.ID,
		GuestSessionID: guestID, BusinessDate: today(), Number: number,
		TotalMinor: total, Currency: "USD", MenuVersion: "7",
		Items: items,
	})
	if err != nil {
		t.Fatalf("seed order %s: %v", number, err)
	}
	return o
}

func TestOrderModel_insertAndHydrate(t *testing.T) {
	conn := testConn(t)
	ctx := context.Background()
	venueID, tableID := newID(), newID()
	session := openSession(t, conn, venueID, tableID)
	guestID := registerGuest(t, conn, session)
	orders := NewOrderModel(conn)

	espressoID, cakeID := newID(), newID()
	optOatID, optShotID := newID(), newID()

	inserted := seedOrder(t, conn, session, guestID, "A-001", []InsertOrderItemParams{
		{
			MenuItemID: espressoID, Name: "Espresso", UnitPriceMinor: 300, Qty: 2,
			Comment: "extra hot", LineTotalMinor: 700,
			Modifiers: []InsertOrderItemModifierParams{
				{OptionID: optOatID, Name: "Oat milk", PriceDeltaMinor: 50},
				{OptionID: optShotID, Name: "Extra shot", PriceDeltaMinor: 0},
			},
		},
		{MenuItemID: cakeID, Name: "Cake", UnitPriceMinor: 500, Qty: 1, LineTotalMinor: 500},
	})

	if inserted.Status != OrderPlaced {
		t.Errorf("a new order is %q, want placed", inserted.Status)
	}
	if inserted.MenuVersion != "7" {
		t.Errorf("menu_version = %q, want the snapshot 7", inserted.MenuVersion)
	}
	if len(inserted.Items) != 2 {
		t.Fatalf("Insert returned %d items, want 2", len(inserted.Items))
	}

	// Read it back fresh: the hydrating reader must reassemble the same
	// aggregate the writer returned.
	loaded, err := orders.FindByID(ctx, venueID, inserted.ID)
	if err != nil {
		t.Fatalf("FindByID: %v", err)
	}
	if len(loaded.Items) != 2 {
		t.Fatalf("hydrated %d items, want 2", len(loaded.Items))
	}

	var espresso *OrderItem
	for i := range loaded.Items {
		if loaded.Items[i].MenuItemID == espressoID {
			espresso = &loaded.Items[i]
		}
	}
	if espresso == nil {
		t.Fatal("the espresso line was not hydrated")
	}
	if espresso.Name != "Espresso" || espresso.Qty != 2 || espresso.LineTotalMinor != 700 {
		t.Errorf("snapshot fields wrong: %+v", espresso)
	}
	if espresso.Comment.String != "extra hot" {
		t.Errorf("comment = %q, want 'extra hot'", espresso.Comment.String)
	}
	if len(espresso.Modifiers) != 2 {
		t.Fatalf("hydrated %d modifiers, want 2", len(espresso.Modifiers))
	}
	if espresso.Modifiers[0].Name == "" || espresso.Modifiers[0].OptionID == "" {
		t.Errorf("modifier snapshot is incomplete: %+v", espresso.Modifiers[0])
	}

	// An empty comment is stored as NULL, not "".
	for i := range loaded.Items {
		if loaded.Items[i].MenuItemID == cakeID && loaded.Items[i].Comment.Valid {
			t.Error("an absent comment should be NULL, not an empty string")
		}
	}
}

// TestOrderModel_hydrateBatchesAcrossOrders guards the N+1 the hydrating
// readers exist to avoid: items and modifiers for a whole page must be
// assembled, and assembled correctly, without a query per order.
func TestOrderModel_hydrateBatchesAcrossOrders(t *testing.T) {
	conn := testConn(t)
	ctx := context.Background()
	venueID, tableID := newID(), newID()
	session := openSession(t, conn, venueID, tableID)
	guestID := registerGuest(t, conn, session)

	want := map[string]int{}
	for i, n := range []string{"A-001", "A-002", "A-003"} {
		items := make([]InsertOrderItemParams, i+1) // 1, 2, then 3 lines
		for j := range items {
			items[j] = InsertOrderItemParams{
				MenuItemID: newID(), Name: "Item", UnitPriceMinor: 100, Qty: 1, LineTotalMinor: 100,
				Modifiers: []InsertOrderItemModifierParams{
					{OptionID: newID(), Name: "Mod", PriceDeltaMinor: 10},
				},
			}
		}
		o := seedOrder(t, conn, session, guestID, n, items)
		want[o.ID] = i + 1
	}

	loaded, err := NewOrderModel(conn).ListByTableSession(ctx, venueID, session.ID)
	if err != nil {
		t.Fatalf("ListByTableSession: %v", err)
	}
	if len(loaded) != 3 {
		t.Fatalf("listed %d orders, want 3", len(loaded))
	}
	for _, o := range loaded {
		if got := len(o.Items); got != want[o.ID] {
			t.Errorf("order %s hydrated %d items, want %d", o.Number, got, want[o.ID])
		}
		for _, it := range o.Items {
			if len(it.Modifiers) != 1 {
				t.Errorf("order %s item %s hydrated %d modifiers, want 1", o.Number, it.ID, len(it.Modifiers))
			}
		}
	}
}

// TestOrderModel_updateStatusIsCompareAndSet is the guarantee the
// transition logic leans on: a stale `from` affects zero rows rather than
// clobbering whatever another actor just wrote.
func TestOrderModel_updateStatusIsCompareAndSet(t *testing.T) {
	conn := testConn(t)
	ctx := context.Background()
	venueID, tableID := newID(), newID()
	session := openSession(t, conn, venueID, tableID)
	guestID := registerGuest(t, conn, session)
	orders := NewOrderModel(conn)

	o := seedOrder(t, conn, session, guestID, "A-001", []InsertOrderItemParams{
		{MenuItemID: newID(), Name: "Tea", UnitPriceMinor: 200, Qty: 1, LineTotalMinor: 200},
	})

	accepted, err := orders.UpdateStatus(ctx, venueID, o.ID, OrderAccepted, OrderPlaced, "")
	if err != nil {
		t.Fatalf("accept: %v", err)
	}
	if accepted.Status != OrderAccepted {
		t.Errorf("status = %q, want accepted", accepted.Status)
	}
	if !accepted.AcceptedAt.Valid {
		t.Error("accepted_at was not stamped")
	}

	// A second actor still holding the old status loses cleanly.
	_, err = orders.UpdateStatus(ctx, venueID, o.ID, OrderInProgress, OrderPlaced, "")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("stale transition error = %v, want ErrNotFound", err)
	}

	ready, err := orders.UpdateStatus(ctx, venueID, o.ID, OrderReady, OrderAccepted, "")
	if err != nil {
		t.Fatalf("ready: %v", err)
	}
	if !ready.ReadyAt.Valid {
		t.Error("ready_at was not stamped")
	}
	// The earlier stamp must survive later transitions.
	if !ready.AcceptedAt.Valid {
		t.Error("accepted_at was cleared by a later transition")
	}

	cancelled, err := orders.UpdateStatus(ctx, venueID, o.ID, OrderCancelled, OrderReady, "guest left")
	if err != nil {
		t.Fatalf("cancel: %v", err)
	}
	if cancelled.CancelledReason.String != "guest left" {
		t.Errorf("cancelled_reason = %q, want 'guest left'", cancelled.CancelledReason.String)
	}

	// Scoping: another venue cannot touch this order.
	_, err = orders.UpdateStatus(ctx, newID(), o.ID, OrderServed, OrderCancelled, "")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("cross-venue transition error = %v, want ErrNotFound", err)
	}
}

func TestOrderModel_itemStatusCountsAndLiveTotal(t *testing.T) {
	conn := testConn(t)
	ctx := context.Background()
	venueID, tableID := newID(), newID()
	session := openSession(t, conn, venueID, tableID)
	guestID := registerGuest(t, conn, session)
	orders := NewOrderModel(conn)

	o := seedOrder(t, conn, session, guestID, "A-001", []InsertOrderItemParams{
		{MenuItemID: newID(), Name: "A", UnitPriceMinor: 100, Qty: 1, LineTotalMinor: 100},
		{MenuItemID: newID(), Name: "B", UnitPriceMinor: 200, Qty: 1, LineTotalMinor: 200},
		{MenuItemID: newID(), Name: "C", UnitPriceMinor: 300, Qty: 1, LineTotalMinor: 300},
	})

	total, err := orders.LiveTotal(ctx, o.ID)
	if err != nil {
		t.Fatalf("live total: %v", err)
	}
	if total != 600 {
		t.Errorf("live total = %d, want 600", total)
	}

	// Cancel the 200 line.
	if _, err := orders.UpdateItemStatus(ctx, o.ID, o.Items[1].ID, ItemCancelled, ItemPlaced); err != nil {
		t.Fatalf("cancel item: %v", err)
	}
	total, err = orders.LiveTotal(ctx, o.ID)
	if err != nil {
		t.Fatalf("live total after cancel: %v", err)
	}
	if total != 400 {
		t.Errorf("live total = %d, want 400 after cancelling a 200 line", total)
	}

	if _, err := orders.UpdateItemStatus(ctx, o.ID, o.Items[0].ID, ItemReady, ItemPlaced); err != nil {
		t.Fatalf("ready item: %v", err)
	}
	counts, err := orders.ItemStatusCounts(ctx, o.ID)
	if err != nil {
		t.Fatalf("counts: %v", err)
	}
	if counts[ItemReady] != 1 || counts[ItemCancelled] != 1 || counts[ItemPlaced] != 1 {
		t.Errorf("counts = %v, want one each of ready/cancelled/placed", counts)
	}

	// A stale item transition loses the same way an order one does.
	_, err = orders.UpdateItemStatus(ctx, o.ID, o.Items[0].ID, ItemCooking, ItemPlaced)
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("stale item transition error = %v, want ErrNotFound", err)
	}

	// Every line cancelled means a zero total, not a NULL one.
	for _, it := range o.Items {
		_, _ = orders.UpdateItemStatus(ctx, o.ID, it.ID, ItemCancelled, ItemPlaced)
		_, _ = orders.UpdateItemStatus(ctx, o.ID, it.ID, ItemCancelled, ItemReady)
	}
	total, err = orders.LiveTotal(ctx, o.ID)
	if err != nil {
		t.Fatalf("live total when everything is cancelled: %v", err)
	}
	if total != 0 {
		t.Errorf("live total = %d, want 0", total)
	}
}

// TestOrderModel_listTicketsPaginatesOldestFirst covers FR-K1's ordering
// and the keyset cursor. Keyset rather than OFFSET matters here because
// the ticket list is re-fetched constantly while new tickets arrive.
func TestOrderModel_listTicketsPaginatesOldestFirst(t *testing.T) {
	conn := testConn(t)
	ctx := context.Background()
	venueID, tableID := newID(), newID()
	session := openSession(t, conn, venueID, tableID)
	guestID := registerGuest(t, conn, session)
	orders := NewOrderModel(conn)

	numbers := []string{"A-001", "A-002", "A-003", "A-004", "A-005"}
	for _, n := range numbers {
		seedOrder(t, conn, session, guestID, n, []InsertOrderItemParams{
			{MenuItemID: newID(), Name: "X", UnitPriceMinor: 100, Qty: 1, LineTotalMinor: 100},
		})
		time.Sleep(2 * time.Millisecond) // distinct placed_at values
	}

	var seen []string
	var cursor Cursor
	for page := 0; page < 5; page++ {
		batch, err := orders.ListTickets(ctx, venueID, nil, cursor, 2)
		if err != nil {
			t.Fatalf("page %d: %v", page, err)
		}
		if len(batch) == 0 {
			break
		}
		for _, o := range batch {
			seen = append(seen, o.Number)
		}
		last := batch[len(batch)-1]
		cursor = Cursor{Timestamp: last.PlacedAt, ID: last.ID}
	}

	if len(seen) != len(numbers) {
		t.Fatalf("paged through %d tickets (%v), want %d", len(seen), seen, len(numbers))
	}
	for i, n := range numbers {
		if seen[i] != n {
			t.Errorf("position %d = %s, want %s (oldest first)", i, seen[i], n)
		}
	}

	// Status filtering: served tickets drop out of the default view.
	served := seen[0]
	all, _ := orders.ListTickets(ctx, venueID, nil, Cursor{}, 50)
	var servedID string
	for _, o := range all {
		if o.Number == served {
			servedID = o.ID
		}
	}
	if _, err := orders.UpdateStatus(ctx, venueID, servedID, OrderAccepted, OrderPlaced, ""); err != nil {
		t.Fatalf("accept: %v", err)
	}
	if _, err := orders.UpdateStatus(ctx, venueID, servedID, OrderReady, OrderAccepted, ""); err != nil {
		t.Fatalf("ready: %v", err)
	}
	if _, err := orders.UpdateStatus(ctx, venueID, servedID, OrderServed, OrderReady, ""); err != nil {
		t.Fatalf("serve: %v", err)
	}

	active, err := orders.ListTickets(ctx, venueID, nil, Cursor{}, 50)
	if err != nil {
		t.Fatalf("active listing: %v", err)
	}
	if len(active) != 4 {
		t.Errorf("active tickets = %d, want 4 after serving one", len(active))
	}

	explicit, err := orders.ListTickets(ctx, venueID, []string{OrderServed}, Cursor{}, 50)
	if err != nil {
		t.Fatalf("served listing: %v", err)
	}
	if len(explicit) != 1 || explicit[0].ID != servedID {
		t.Errorf("explicit served filter returned %d rows, want just the served one", len(explicit))
	}
}

// TestOrderModel_cursorRoundTrip covers the encoding, including the
// zero-value cursor that selects the first page.
func TestOrderModel_cursorRoundTrip(t *testing.T) {
	var zero Cursor
	if zero.Encode() != "" {
		t.Errorf("a zero cursor encodes to %q, want empty", zero.Encode())
	}
	decoded, err := DecodeCursor("")
	if err != nil || decoded != (Cursor{}) {
		t.Errorf("DecodeCursor(\"\") = %v, %v; want the zero cursor", decoded, err)
	}
	// The zero cursor must produce SQL-safe bounds, not "" for a uuid.
	if zero.idOrZero() != zeroUUID {
		t.Errorf("idOrZero = %q, want the all-zeros uuid", zero.idOrZero())
	}
	if !zero.timestampOrEpoch().Equal(epoch) {
		t.Errorf("timestampOrEpoch = %v, want the epoch", zero.timestampOrEpoch())
	}

	original := Cursor{Timestamp: time.Now().UTC().Truncate(time.Nanosecond), ID: newID()}
	round, err := DecodeCursor(original.Encode())
	if err != nil {
		t.Fatalf("round trip: %v", err)
	}
	if round.ID != original.ID || !round.Timestamp.Equal(original.Timestamp) {
		t.Errorf("round trip changed the cursor: %v -> %v", original, round)
	}

	for _, bad := range []string{"!!!", "bm90LWEtY3Vyc29y", "MjAyNi0wMS0wMQ"} {
		if _, err := DecodeCursor(bad); err == nil {
			t.Errorf("DecodeCursor(%q) succeeded, want an error", bad)
		}
	}
}

// ---------------------------------------------------------------------
// Service requests (FR-S1..S4)
// ---------------------------------------------------------------------

// TestServiceRequestModel_createOrGetOpen is FR-S2's rate limit: a repeat
// returns the existing open request rather than creating a second one.
// The guarantee is service_requests_one_open_per_table_type_idx.
func TestServiceRequestModel_createOrGetOpen(t *testing.T) {
	conn := testConn(t)
	ctx := context.Background()
	venueID, tableID := newID(), newID()
	session := openSession(t, conn, venueID, tableID)
	requests := NewServiceRequestModel(conn)

	first, created, err := requests.CreateOrGetOpen(ctx, venueID, tableID, session.ID, RequestCallWaiter, "", 15*time.Minute)
	if err != nil {
		t.Fatalf("first create: %v", err)
	}
	if !created {
		t.Error("the first request should report created=true")
	}
	if first.Status != RequestOpen {
		t.Errorf("status = %q, want open", first.Status)
	}

	second, created, err := requests.CreateOrGetOpen(ctx, venueID, tableID, session.ID, RequestCallWaiter, "", 15*time.Minute)
	if err != nil {
		t.Fatalf("repeat: %v", err)
	}
	if created {
		t.Error("a repeat should report created=false")
	}
	if second.ID != first.ID {
		t.Errorf("a repeat created a new request %s instead of returning %s", second.ID, first.ID)
	}

	// A different *type* at the same table is a separate request.
	bill, created, err := requests.CreateOrGetOpen(ctx, venueID, tableID, session.ID, RequestRequestBill, "cash please", 15*time.Minute)
	if err != nil {
		t.Fatalf("bill request: %v", err)
	}
	if !created || bill.ID == first.ID {
		t.Error("request_bill should be its own request, separate from call_waiter")
	}
	if bill.Note.String != "cash please" {
		t.Errorf("note = %q, want the FR-S7 payment hint", bill.Note.String)
	}

	// Once resolved, the table can call again.
	if _, err := requests.UpdateStatus(ctx, venueID, first.ID, RequestResolved, RequestOpen); err != nil {
		t.Fatalf("resolve: %v", err)
	}
	third, created, err := requests.CreateOrGetOpen(ctx, venueID, tableID, session.ID, RequestCallWaiter, "", 15*time.Minute)
	if err != nil {
		t.Fatalf("create after resolve: %v", err)
	}
	if !created || third.ID == first.ID {
		t.Error("a new request should be possible once the previous one is resolved")
	}
}

func TestServiceRequestModel_transitionStampsTimestamps(t *testing.T) {
	conn := testConn(t)
	ctx := context.Background()
	venueID, tableID := newID(), newID()
	session := openSession(t, conn, venueID, tableID)
	requests := NewServiceRequestModel(conn)

	r, _, err := requests.CreateOrGetOpen(ctx, venueID, tableID, session.ID, RequestCallWaiter, "", 15*time.Minute)
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	ack, err := requests.UpdateStatus(ctx, venueID, r.ID, RequestAcknowledged, RequestOpen)
	if err != nil {
		t.Fatalf("acknowledge: %v", err)
	}
	if !ack.AcknowledgedAt.Valid {
		t.Error("acknowledged_at was not stamped")
	}
	if ack.ResolvedAt.Valid {
		t.Error("resolved_at was stamped too early")
	}

	resolved, err := requests.UpdateStatus(ctx, venueID, r.ID, RequestResolved, RequestAcknowledged)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if !resolved.ResolvedAt.Valid {
		t.Error("resolved_at was not stamped")
	}
	if !resolved.AcknowledgedAt.Valid || !resolved.AcknowledgedAt.Time.Equal(ack.AcknowledgedAt.Time) {
		t.Error("resolving overwrote the earlier acknowledgement timestamp")
	}

	// Compare-and-set: a stale transition affects nothing.
	_, err = requests.UpdateStatus(ctx, venueID, r.ID, RequestResolved, RequestOpen)
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("stale transition error = %v, want ErrNotFound", err)
	}
}

// TestServiceRequestModel_expireOverdue is FR-S4.
func TestServiceRequestModel_expireOverdue(t *testing.T) {
	conn := testConn(t)
	ctx := context.Background()
	venueID := newID()
	staleTable, freshTable := newID(), newID()
	staleSession := openSession(t, conn, venueID, staleTable)
	freshSession := openSession(t, conn, venueID, freshTable)
	requests := NewServiceRequestModel(conn)

	// A negative TTL is the deterministic way to express "already
	// overdue" — the expiry is computed by Postgres, so no clock skew
	// between the test process and the server can make it flaky.
	stale, _, err := requests.CreateOrGetOpen(ctx, venueID, staleTable, staleSession.ID, RequestCallWaiter, "", -time.Minute)
	if err != nil {
		t.Fatalf("create stale: %v", err)
	}
	fresh, _, err := requests.CreateOrGetOpen(ctx, venueID, freshTable, freshSession.ID, RequestCallWaiter, "", 15*time.Minute)
	if err != nil {
		t.Fatalf("create fresh: %v", err)
	}

	n, err := requests.ExpireOverdue(ctx, venueID)
	if err != nil {
		t.Fatalf("expire: %v", err)
	}
	if n != 1 {
		t.Errorf("expired %d requests, want 1", n)
	}

	after, err := requests.FindByID(ctx, venueID, stale.ID)
	if err != nil {
		t.Fatalf("reload stale: %v", err)
	}
	if after.Status != RequestExpired {
		t.Errorf("stale request status = %q, want expired", after.Status)
	}

	stillOpen, err := requests.FindByID(ctx, venueID, fresh.ID)
	if err != nil {
		t.Fatalf("reload fresh: %v", err)
	}
	if stillOpen.Status != RequestOpen {
		t.Errorf("fresh request status = %q, want open", stillOpen.Status)
	}

	// Idempotent: a second sweep changes nothing.
	if n, err := requests.ExpireOverdue(ctx, venueID); err != nil || n != 0 {
		t.Errorf("second sweep expired %d (err %v), want 0", n, err)
	}
}

func TestServiceRequestModel_resolveOpenForSession(t *testing.T) {
	conn := testConn(t)
	ctx := context.Background()
	venueID, tableID := newID(), newID()
	session := openSession(t, conn, venueID, tableID)
	requests := NewServiceRequestModel(conn)

	if _, _, err := requests.CreateOrGetOpen(ctx, venueID, tableID, session.ID, RequestCallWaiter, "", 15*time.Minute); err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, _, err := requests.CreateOrGetOpen(ctx, venueID, tableID, session.ID, RequestRequestBill, "", 15*time.Minute); err != nil {
		t.Fatalf("create: %v", err)
	}

	resolved, err := requests.ResolveOpenForSession(ctx, session.ID)
	if err != nil {
		t.Fatalf("resolve open: %v", err)
	}
	if len(resolved) != 2 {
		t.Fatalf("resolved %d requests, want 2", len(resolved))
	}
	for _, r := range resolved {
		if r.Status != RequestResolved || !r.ResolvedAt.Valid {
			t.Errorf("request %s was not properly resolved: %+v", r.ID, r)
		}
	}

	open, err := requests.List(ctx, venueID, nil)
	if err != nil {
		t.Fatalf("list open: %v", err)
	}
	if len(open) != 0 {
		t.Errorf("%d requests are still open after closing the session", len(open))
	}
}

// ---------------------------------------------------------------------
// Idempotency (docs/TZ.md §7.4)
// ---------------------------------------------------------------------

func TestIdempotencyModel_claimCompleteAndReplay(t *testing.T) {
	conn := testConn(t)
	ctx := context.Background()
	venueID := newID()
	idem := NewIdempotencyModel(conn)
	key := newID()
	fp := Fingerprint("venue", "table", "one latte")
	t.Cleanup(func() { _, _ = conn.ExecCtx(ctx, `DELETE FROM idempotency_keys WHERE venue_id = $1`, venueID) })

	row, owned, err := idem.Claim(ctx, venueID, "order.CreateOrder", key, fp)
	if err != nil {
		t.Fatalf("first claim: %v", err)
	}
	if !owned {
		t.Fatal("the first claim should own the key")
	}
	if !row.InProgress {
		t.Error("a fresh claim should be marked in progress")
	}

	if err := idem.Complete(ctx, venueID, "order.CreateOrder", key, 200, []byte(`{"id":"abc"}`)); err != nil {
		t.Fatalf("complete: %v", err)
	}

	prior, owned, err := idem.Claim(ctx, venueID, "order.CreateOrder", key, fp)
	if err != nil {
		t.Fatalf("replay claim: %v", err)
	}
	if owned {
		t.Fatal("a replay must not re-own the key")
	}
	if prior.InProgress {
		t.Error("a completed record should not still be in progress")
	}
	if string(prior.ResponseBody) != `{"id": "abc"}` && string(prior.ResponseBody) != `{"id":"abc"}` {
		t.Errorf("stored response = %s, want the original body", prior.ResponseBody)
	}
	if prior.StatusCode != 200 {
		t.Errorf("status_code = %d, want 200", prior.StatusCode)
	}

	// Find is the read-only peek used by the fast path.
	found, err := idem.Find(ctx, venueID, "order.CreateOrder", key)
	if err != nil {
		t.Fatalf("find: %v", err)
	}
	if found.Fingerprint != fp {
		t.Errorf("fingerprint = %q, want %q", found.Fingerprint, fp)
	}
}

// TestIdempotencyModel_scopeIsVenueEndpointKey checks all three parts of
// the key actually participate: the same key under a different venue or
// endpoint is a different request (§7.4).
func TestIdempotencyModel_scopeIsVenueEndpointKey(t *testing.T) {
	conn := testConn(t)
	ctx := context.Background()
	venueA, venueB := newID(), newID()
	idem := NewIdempotencyModel(conn)
	key := newID()
	fp := Fingerprint("x")
	t.Cleanup(func() {
		_, _ = conn.ExecCtx(ctx, `DELETE FROM idempotency_keys WHERE venue_id = ANY($1::uuid[])`,
			pgTextArrayLiteral([]string{venueA, venueB}))
	})

	if _, owned, err := idem.Claim(ctx, venueA, "order.CreateOrder", key, fp); err != nil || !owned {
		t.Fatalf("claim in venue A: owned=%v err=%v", owned, err)
	}
	if _, owned, err := idem.Claim(ctx, venueB, "order.CreateOrder", key, fp); err != nil || !owned {
		t.Fatalf("the same key in another venue should be free: owned=%v err=%v", owned, err)
	}
	if _, owned, err := idem.Claim(ctx, venueA, "order.TransitionOrder", key, fp); err != nil || !owned {
		t.Fatalf("the same key on another endpoint should be free: owned=%v err=%v", owned, err)
	}
	if _, owned, err := idem.Claim(ctx, venueA, "order.CreateOrder", key, fp); err != nil || owned {
		t.Fatalf("re-claiming the same triple should not own it: owned=%v err=%v", owned, err)
	}
}

func TestFingerprintSeparatesFields(t *testing.T) {
	if Fingerprint("ab", "c") == Fingerprint("a", "bc") {
		t.Error("adjacent fields ran together: Fingerprint needs a domain separator")
	}
	first, second := Fingerprint("a", "b"), Fingerprint("a", "b")
	if first != second {
		t.Errorf("Fingerprint is not deterministic: %s vs %s", first, second)
	}
	if Fingerprint() == "" {
		t.Error("Fingerprint() should still produce a hash")
	}
}

func TestIdempotencyModel_purge(t *testing.T) {
	conn := testConn(t)
	ctx := context.Background()
	venueID := newID()
	idem := NewIdempotencyModel(conn)
	t.Cleanup(func() { _, _ = conn.ExecCtx(ctx, `DELETE FROM idempotency_keys WHERE venue_id = $1`, venueID) })

	if _, _, err := idem.Claim(ctx, venueID, "order.CreateOrder", newID(), Fingerprint("x")); err != nil {
		t.Fatalf("claim: %v", err)
	}
	// Age it past the TTL. Done in SQL so the comparison happens entirely
	// on the server's clock.
	if _, err := conn.ExecCtx(ctx,
		`UPDATE idempotency_keys SET created_at = now() - interval '48 hours' WHERE venue_id = $1`, venueID); err != nil {
		t.Fatalf("age the row: %v", err)
	}

	n, err := idem.PurgeOlderThan(ctx, 24*time.Hour)
	if err != nil {
		t.Fatalf("purge: %v", err)
	}
	if n < 1 {
		t.Errorf("purged %d rows, want at least the aged one", n)
	}
}

// ---------------------------------------------------------------------
// Reporting (FR-A3)
// ---------------------------------------------------------------------

func TestReportModel_dayTotalsAndTopItems(t *testing.T) {
	conn := testConn(t)
	ctx := context.Background()
	venueID, tableID := newID(), newID()
	session := openSession(t, conn, venueID, tableID)
	guestID := registerGuest(t, conn, session)
	orders := NewOrderModel(conn)
	reports := NewReportModel(conn)
	date := today()

	latteID, cakeID := newID(), newID()

	// Two live orders...
	seedOrder(t, conn, session, guestID, "A-001", []InsertOrderItemParams{
		{MenuItemID: latteID, Name: "Latte", UnitPriceMinor: 400, Qty: 2, LineTotalMinor: 800},
		{MenuItemID: cakeID, Name: "Cake", UnitPriceMinor: 600, Qty: 1, LineTotalMinor: 600},
	})
	seedOrder(t, conn, session, guestID, "A-002", []InsertOrderItemParams{
		{MenuItemID: latteID, Name: "Latte", UnitPriceMinor: 400, Qty: 1, LineTotalMinor: 400},
	})
	// ...and one cancelled, which must not appear anywhere in the report.
	cancelled := seedOrder(t, conn, session, guestID, "A-003", []InsertOrderItemParams{
		{MenuItemID: cakeID, Name: "Cake", UnitPriceMinor: 600, Qty: 10, LineTotalMinor: 6000},
	})
	if _, err := orders.UpdateStatus(ctx, venueID, cancelled.ID, OrderCancelled, OrderPlaced, "test"); err != nil {
		t.Fatalf("cancel: %v", err)
	}

	totals, err := reports.DayTotals(ctx, venueID, date)
	if err != nil {
		t.Fatalf("day totals: %v", err)
	}
	if totals.OrdersCount != 2 {
		t.Errorf("orders_count = %d, want 2 (the cancelled one excluded)", totals.OrdersCount)
	}
	if totals.RevenueMinor != 1800 {
		t.Errorf("revenue = %d, want 1800", totals.RevenueMinor)
	}
	if totals.Currency.String != "USD" {
		t.Errorf("currency = %q, want USD", totals.Currency.String)
	}
	// Nothing was accepted, so the timing averages have no rows to
	// average and must be NULL rather than a misleading zero.
	if totals.AvgAcceptSecs.Valid {
		t.Errorf("avg_accept_seconds = %v, want NULL when nothing was accepted", totals.AvgAcceptSecs.Float64)
	}

	top, err := reports.TopItems(ctx, venueID, date, 10)
	if err != nil {
		t.Fatalf("top items: %v", err)
	}
	if len(top) != 2 {
		t.Fatalf("top items = %d, want 2", len(top))
	}
	if top[0].MenuItemID != latteID {
		t.Errorf("best seller = %s, want the latte (1200 > 600)", top[0].Name)
	}
	if top[0].QtySold != 3 || top[0].RevenueMinor != 1200 {
		t.Errorf("latte: qty=%d revenue=%d, want 3 and 1200", top[0].QtySold, top[0].RevenueMinor)
	}
	if top[1].QtySold != 1 || top[1].RevenueMinor != 600 {
		t.Errorf("cake: qty=%d revenue=%d, want 1 and 600 (the cancelled order excluded)",
			top[1].QtySold, top[1].RevenueMinor)
	}

	// An empty day reports zeros and a NULL currency, not an error.
	empty, err := reports.DayTotals(ctx, newID(), date)
	if err != nil {
		t.Fatalf("empty day: %v", err)
	}
	if empty.OrdersCount != 0 || empty.RevenueMinor != 0 || empty.Currency.Valid {
		t.Errorf("empty day = %+v, want zeros and a NULL currency", empty)
	}
}

func TestReportModel_timingAveragesUseTransitionStamps(t *testing.T) {
	conn := testConn(t)
	ctx := context.Background()
	venueID, tableID := newID(), newID()
	session := openSession(t, conn, venueID, tableID)
	guestID := registerGuest(t, conn, session)
	reports := NewReportModel(conn)

	o := seedOrder(t, conn, session, guestID, "A-001", []InsertOrderItemParams{
		{MenuItemID: newID(), Name: "Soup", UnitPriceMinor: 500, Qty: 1, LineTotalMinor: 500},
	})

	// Backdate placed_at and stamp the transitions explicitly so the
	// averages are exact rather than dependent on test wall time.
	if _, err := conn.ExecCtx(ctx, `
		UPDATE orders SET
			placed_at   = now() - interval '10 minutes',
			accepted_at = now() - interval '9 minutes',
			ready_at    = now() - interval '4 minutes',
			status      = 'ready'
		WHERE id = $1`, o.ID); err != nil {
		t.Fatalf("backdate: %v", err)
	}

	totals, err := reports.DayTotals(ctx, venueID, today())
	if err != nil {
		t.Fatalf("day totals: %v", err)
	}
	if !totals.AvgAcceptSecs.Valid {
		t.Fatal("avg_accept_seconds is NULL despite a stamped accepted_at")
	}
	if got := totals.AvgAcceptSecs.Float64; got < 59 || got > 61 {
		t.Errorf("avg_accept_seconds = %v, want ~60", got)
	}
	if got := totals.AvgCookSecs.Float64; got < 299 || got > 301 {
		t.Errorf("avg_cook_seconds = %v, want ~300", got)
	}
}

// ---------------------------------------------------------------------
// Outbox
// ---------------------------------------------------------------------

func TestOutboxModel_insertMarshalsPayload(t *testing.T) {
	conn := testConn(t)
	ctx := context.Background()
	venueID := newID()
	t.Cleanup(func() { _, _ = conn.ExecCtx(ctx, `DELETE FROM outbox WHERE venue_id = $1`, venueID) })

	payload := struct {
		OrderID string `json:"order_id"`
		Total   int64  `json:"total_minor"`
	}{OrderID: "abc", Total: 1234}

	if err := NewOutboxModel(conn).Insert(ctx, "order.placed", venueID, TopicOrder, "abc", payload, "trace-1"); err != nil {
		t.Fatalf("insert: %v", err)
	}

	var got struct {
		EventType string `db:"event_type"`
		Topic     string `db:"topic"`
		Key       string `db:"partition_key"`
		Payload   string `db:"payload"`
		TraceID   string `db:"trace_id"`
	}
	if err := conn.QueryRowCtx(ctx, &got,
		`SELECT event_type, topic, partition_key, payload::text AS payload, trace_id
		 FROM outbox WHERE venue_id = $1`, venueID); err != nil {
		t.Fatalf("read back: %v", err)
	}
	if got.EventType != "order.placed" || got.Topic != TopicOrder || got.Key != "abc" || got.TraceID != "trace-1" {
		t.Errorf("envelope columns wrong: %+v", got)
	}
	if !strings.Contains(got.Payload, `"order_id"`) || !strings.Contains(got.Payload, "1234") {
		t.Errorf("payload = %s, want the marshaled struct", got.Payload)
	}
}
