package model

import (
	"context"
	"database/sql"
	"strings"
	"time"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

// Order status values, mirroring the CHECK constraint on orders.status.
const (
	OrderPlaced     = "placed"
	OrderAccepted   = "accepted"
	OrderInProgress = "in_progress"
	OrderReady      = "ready"
	OrderServed     = "served"
	OrderCancelled  = "cancelled"
)

// Order item status values, mirroring order_items.status.
const (
	ItemPlaced    = "placed"
	ItemCooking   = "cooking"
	ItemReady     = "ready"
	ItemCancelled = "cancelled"
)

// ActiveOrderStatuses is ListTickets' default filter (FR-K1: the ticket
// list is the work still in front of the kitchen).
var ActiveOrderStatuses = []string{OrderPlaced, OrderAccepted, OrderInProgress, OrderReady}

// Order mirrors order_db.orders. total_minor, currency and menu_version
// are snapshots fixed at placement so a later menu edit never rewrites
// history (docs/TZ.md §6).
type Order struct {
	ID              string         `db:"id"`
	VenueID         string         `db:"venue_id"`
	TableID         string         `db:"table_id"`
	TableSessionID  string         `db:"table_session_id"`
	GuestSessionID  string         `db:"guest_session_id"`
	BusinessDate    time.Time      `db:"business_date"`
	Number          string         `db:"number"`
	Status          string         `db:"status"`
	TotalMinor      int64          `db:"total_minor"`
	Currency        string         `db:"currency"`
	MenuVersion     string         `db:"menu_version"`
	CancelledReason sql.NullString `db:"cancelled_reason"`
	PlacedAt        time.Time      `db:"placed_at"`
	UpdatedAt       time.Time      `db:"updated_at"`

	// Transition stamps (migration 000003). Nullable: an order that never
	// reached a state has no timestamp for it, and orders placed before
	// that migration have none at all — GetDayReport's averages skip them
	// rather than counting them as zero-duration.
	AcceptedAt sql.NullTime `db:"accepted_at"`
	ReadyAt    sql.NullTime `db:"ready_at"`
	ServedAt   sql.NullTime `db:"served_at"`

	// Items is not a column: it is filled by the hydrating readers below
	// (FindByID, ListByTableSession, ListTickets) so the logic layer never
	// has to assemble an aggregate itself.
	Items []OrderItem `db:"-"`
}

// OrderItem mirrors order_db.order_items.
type OrderItem struct {
	ID             string         `db:"id"`
	OrderID        string         `db:"order_id"`
	MenuItemID     string         `db:"menu_item_id"`
	Name           string         `db:"name"`
	UnitPriceMinor int64          `db:"unit_price_minor"`
	Qty            int32          `db:"qty"`
	Comment        sql.NullString `db:"comment"`
	Status         string         `db:"status"`
	LineTotalMinor int64          `db:"line_total_minor"`
	CreatedAt      time.Time      `db:"created_at"`
	UpdatedAt      time.Time      `db:"updated_at"`

	Modifiers []OrderItemModifier `db:"-"`
}

// OrderItemModifier mirrors order_db.order_item_modifiers. Every field
// besides the ids is a snapshot taken at placement.
type OrderItemModifier struct {
	ID              string `db:"id"`
	OrderItemID     string `db:"order_item_id"`
	OptionID        string `db:"option_id"`
	Name            string `db:"name"`
	PriceDeltaMinor int64  `db:"price_delta_minor"`
}

type OrderModel struct {
	conn sqlx.Session
}

func NewOrderModel(conn sqlx.Session) *OrderModel {
	return &OrderModel{conn: conn}
}

const orderCols = `id, venue_id, table_id, table_session_id, guest_session_id, business_date,
	number, status, total_minor, currency, menu_version, cancelled_reason, placed_at, updated_at,
	accepted_at, ready_at, served_at`

const orderItemCols = `id, order_id, menu_item_id, name, unit_price_minor, qty, comment, status,
	line_total_minor, created_at, updated_at`

const orderItemModifierCols = `id, order_item_id, option_id, name, price_delta_minor`

// NewOrderNumberSeq atomically reserves the next per-venue, per-business-day
// sequence number (FR-O8). The upsert is a single statement, so it needs
// no SELECT ... FOR UPDATE and cannot hand the same seq to two concurrent
// orders — see the migration's comment on order_number_counters.
//
// businessDate is the venue-local business day, computed by the caller
// from the venue's timezone; this model never guesses it.
func (m *OrderModel) NewOrderNumberSeq(ctx context.Context, venueID string, businessDate time.Time) (int, error) {
	var seq int
	err := m.conn.QueryRowCtx(ctx, &seq, `
		INSERT INTO order_number_counters (venue_id, business_date, next_seq)
		VALUES ($1, $2, 1)
		ON CONFLICT (venue_id, business_date)
		DO UPDATE SET next_seq = order_number_counters.next_seq + 1
		RETURNING next_seq`,
		venueID, businessDate)
	return seq, err
}

// InsertOrderParams is the whole aggregate as one value, because an order
// with no items is never valid and the two must be inserted together.
type InsertOrderParams struct {
	VenueID        string
	TableID        string
	TableSessionID string
	GuestSessionID string
	BusinessDate   time.Time
	Number         string
	TotalMinor     int64
	Currency       string
	MenuVersion    string
	Items          []InsertOrderItemParams
}

type InsertOrderItemParams struct {
	MenuItemID     string
	Name           string
	UnitPriceMinor int64
	Qty            int32
	Comment        string
	LineTotalMinor int64
	Modifiers      []InsertOrderItemModifierParams
}

type InsertOrderItemModifierParams struct {
	OptionID        string
	Name            string
	PriceDeltaMinor int64
}

// Insert writes the order, its items and their modifier snapshots. It
// must run inside a transaction (the caller's, alongside the table
// session upsert and the outbox row) — it opens none of its own.
func (m *OrderModel) Insert(ctx context.Context, p InsertOrderParams) (*Order, error) {
	var o Order
	err := m.conn.QueryRowCtx(ctx, &o, `
		INSERT INTO orders (venue_id, table_id, table_session_id, guest_session_id, business_date,
			number, total_minor, currency, menu_version)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING `+orderCols,
		p.VenueID, p.TableID, p.TableSessionID, p.GuestSessionID, p.BusinessDate,
		p.Number, p.TotalMinor, p.Currency, p.MenuVersion)
	if err != nil {
		return nil, err
	}

	for _, ip := range p.Items {
		var comment any
		if ip.Comment != "" {
			comment = ip.Comment
		}
		var item OrderItem
		err := m.conn.QueryRowCtx(ctx, &item, `
			INSERT INTO order_items (order_id, menu_item_id, name, unit_price_minor, qty, comment, line_total_minor)
			VALUES ($1, $2, $3, $4, $5, $6, $7)
			RETURNING `+orderItemCols,
			o.ID, ip.MenuItemID, ip.Name, ip.UnitPriceMinor, ip.Qty, comment, ip.LineTotalMinor)
		if err != nil {
			return nil, err
		}
		for _, mp := range ip.Modifiers {
			var mod OrderItemModifier
			err := m.conn.QueryRowCtx(ctx, &mod, `
				INSERT INTO order_item_modifiers (order_item_id, option_id, name, price_delta_minor)
				VALUES ($1, $2, $3, $4)
				RETURNING `+orderItemModifierCols,
				item.ID, mp.OptionID, mp.Name, mp.PriceDeltaMinor)
			if err != nil {
				return nil, err
			}
			item.Modifiers = append(item.Modifiers, mod)
		}
		o.Items = append(o.Items, item)
	}
	return &o, nil
}

// FindByID returns one fully hydrated order.
func (m *OrderModel) FindByID(ctx context.Context, venueID, id string) (*Order, error) {
	var o Order
	err := m.conn.QueryRowCtx(ctx, &o,
		`SELECT `+orderCols+` FROM orders WHERE id = $1 AND venue_id = $2`, id, venueID)
	if err != nil {
		return nil, err
	}
	if err := m.hydrate(ctx, []*Order{&o}); err != nil {
		return nil, err
	}
	return &o, nil
}

// FindByIDForUpdate takes a row lock on the order before a transition
// reads-then-writes it, so two staff pressing "ready" at the same moment
// serialize instead of both seeing 'accepted' and both writing.
func (m *OrderModel) FindByIDForUpdate(ctx context.Context, venueID, id string) (*Order, error) {
	var o Order
	err := m.conn.QueryRowCtx(ctx, &o,
		`SELECT `+orderCols+` FROM orders WHERE id = $1 AND venue_id = $2 FOR UPDATE`, id, venueID)
	if err != nil {
		return nil, err
	}
	return &o, nil
}

// ListByTableSession backs the guest's order list and GetBill (FR-O10,
// FR-S5): every order on the session, oldest first.
func (m *OrderModel) ListByTableSession(ctx context.Context, venueID, tableSessionID string) ([]Order, error) {
	var rows []Order
	err := m.conn.QueryRowsCtx(ctx, &rows,
		`SELECT `+orderCols+` FROM orders
		 WHERE venue_id = $1 AND table_session_id = $2
		 ORDER BY placed_at, id`,
		venueID, tableSessionID)
	if err != nil {
		return nil, err
	}
	return rows, m.hydrate(ctx, ptrs(rows))
}

// ListTickets is the KDS ticket list (FR-K1): oldest first, keyset
// paginated, filtered to the given statuses (defaulting to the active
// subset when the caller passes none).
func (m *OrderModel) ListTickets(ctx context.Context, venueID string, statuses []string, after Cursor, limit int) ([]Order, error) {
	if len(statuses) == 0 {
		statuses = ActiveOrderStatuses
	}
	var rows []Order
	err := m.conn.QueryRowsCtx(ctx, &rows, `
		SELECT `+orderCols+` FROM orders
		WHERE venue_id = $1
		  AND status = ANY($2::text[])
		  AND (placed_at, id) > ($3, $4)
		ORDER BY placed_at, id
		LIMIT $5`,
		venueID, pgTextArrayLiteral(statuses), after.timestampOrEpoch(), after.idOrZero(), limit)
	if err != nil {
		return nil, err
	}
	return rows, m.hydrate(ctx, ptrs(rows))
}

// UpdateStatus applies a transition. The `AND status = $4` guard makes
// the write itself the concurrency check: if another actor moved the
// order first, this affects zero rows and returns ErrNotFound, which the
// logic layer reports as a failed precondition rather than overwriting.
func (m *OrderModel) UpdateStatus(ctx context.Context, venueID, id, to, from, reason string) (*Order, error) {
	var cancelReason any
	if reason != "" {
		cancelReason = reason
	}
	var o Order
	err := m.conn.QueryRowCtx(ctx, &o, `
		UPDATE orders SET
			status = $3,
			cancelled_reason = COALESCE($5, cancelled_reason),
			accepted_at = CASE WHEN $3 = 'accepted' THEN COALESCE(accepted_at, now()) ELSE accepted_at END,
			ready_at    = CASE WHEN $3 = 'ready'    THEN COALESCE(ready_at, now())    ELSE ready_at END,
			served_at   = CASE WHEN $3 = 'served'   THEN COALESCE(served_at, now())   ELSE served_at END
		WHERE id = $1 AND venue_id = $2 AND status = $4
		RETURNING `+orderCols,
		id, venueID, to, from, cancelReason)
	if err != nil {
		return nil, err
	}
	return &o, nil
}

// FindItem loads one line of an order, scoped through the order so a
// caller can't transition an item belonging to a different venue.
func (m *OrderModel) FindItem(ctx context.Context, orderID, itemID string) (*OrderItem, error) {
	var item OrderItem
	err := m.conn.QueryRowCtx(ctx, &item,
		`SELECT `+orderItemCols+` FROM order_items WHERE id = $1 AND order_id = $2`, itemID, orderID)
	if err != nil {
		return nil, err
	}
	return &item, nil
}

// UpdateItemStatus carries the same compare-and-set guard as UpdateStatus.
func (m *OrderModel) UpdateItemStatus(ctx context.Context, orderID, itemID, to, from string) (*OrderItem, error) {
	var item OrderItem
	err := m.conn.QueryRowCtx(ctx, &item, `
		UPDATE order_items SET status = $3
		WHERE id = $1 AND order_id = $2 AND status = $4
		RETURNING `+orderItemCols,
		itemID, orderID, to, from)
	if err != nil {
		return nil, err
	}
	return &item, nil
}

// ItemStatusCounts backs FR-K5 ("an order becomes ready automatically when
// all non-cancelled items are ready") and the cancel-the-last-live-item
// case. Returning counts rather than a bool keeps the rule in the logic
// layer where it is testable in isolation.
func (m *OrderModel) ItemStatusCounts(ctx context.Context, orderID string) (map[string]int, error) {
	var rows []struct {
		Status string `db:"status"`
		N      int    `db:"n"`
	}
	err := m.conn.QueryRowsCtx(ctx, &rows,
		`SELECT status, count(*) AS n FROM order_items WHERE order_id = $1 GROUP BY status`, orderID)
	if err != nil {
		return nil, err
	}
	counts := make(map[string]int, len(rows))
	for _, r := range rows {
		counts[r.Status] = r.N
	}
	return counts, nil
}

// SetTotal rewrites an order's total after a line is cancelled. Orders
// are immutable in every other respect; this exists only so a cancelled
// line stops being billed.
func (m *OrderModel) SetTotal(ctx context.Context, orderID string, totalMinor int64) error {
	_, err := m.conn.ExecCtx(ctx, `UPDATE orders SET total_minor = $2 WHERE id = $1`, orderID, totalMinor)
	return err
}

// LiveTotal sums the non-cancelled lines of an order — the value SetTotal
// should be given after a line-level cancellation.
//
// COALESCE, not a nullable destination: sum() over zero rows is NULL, and
// an order with every line cancelled is a real case, not an error. (It
// also has to be COALESCE rather than scanning into sql.NullInt64 — a
// top-level scan destination has to be a plain type for go-zero's row
// mapper, which reports "not matching destination to scan" otherwise.)
func (m *OrderModel) LiveTotal(ctx context.Context, orderID string) (int64, error) {
	var total int64
	err := m.conn.QueryRowCtx(ctx, &total,
		`SELECT COALESCE(sum(line_total_minor), 0) FROM order_items
		 WHERE order_id = $1 AND status <> 'cancelled'`, orderID)
	if err != nil {
		return 0, err
	}
	return total, nil
}

// hydrate fills Items (and their Modifiers) for a page of orders using
// one query per level rather than one per order — the same shape as
// catalog's menu hydration.
func (m *OrderModel) hydrate(ctx context.Context, orders []*Order) error {
	if len(orders) == 0 {
		return nil
	}
	orderIDs := make([]string, len(orders))
	byID := make(map[string]*Order, len(orders))
	for i, o := range orders {
		orderIDs[i] = o.ID
		byID[o.ID] = o
	}

	var items []OrderItem
	err := m.conn.QueryRowsCtx(ctx, &items,
		`SELECT `+orderItemCols+` FROM order_items WHERE order_id = ANY($1::uuid[]) ORDER BY order_id, created_at, id`,
		pgTextArrayLiteral(orderIDs))
	if err != nil {
		return err
	}
	if len(items) == 0 {
		return nil
	}

	itemIDs := make([]string, len(items))
	for i := range items {
		itemIDs[i] = items[i].ID
	}
	var mods []OrderItemModifier
	err = m.conn.QueryRowsCtx(ctx, &mods,
		`SELECT `+orderItemModifierCols+` FROM order_item_modifiers WHERE order_item_id = ANY($1::uuid[]) ORDER BY order_item_id, id`,
		pgTextArrayLiteral(itemIDs))
	if err != nil {
		return err
	}
	modsByItem := make(map[string][]OrderItemModifier, len(items))
	for _, md := range mods {
		modsByItem[md.OrderItemID] = append(modsByItem[md.OrderItemID], md)
	}

	for i := range items {
		items[i].Modifiers = modsByItem[items[i].ID]
		if o, ok := byID[items[i].OrderID]; ok {
			o.Items = append(o.Items, items[i])
		}
	}
	return nil
}

// ptrs adapts a freshly-read slice for hydrate, which mutates in place.
func ptrs(orders []Order) []*Order {
	out := make([]*Order, len(orders))
	for i := range orders {
		out[i] = &orders[i]
	}
	return out
}

// pgTextArrayLiteral renders a Go slice as a Postgres array literal.
// pgx's stdlib driver passes []string through as an opaque driver.Value
// it cannot encode, so array parameters go over the wire as text and are
// cast in SQL ($1::uuid[], $1::text[]) — the same approach the catalog
// service's model package documents.
func pgTextArrayLiteral(vals []string) string {
	if len(vals) == 0 {
		return "{}"
	}
	var b strings.Builder
	b.WriteByte('{')
	for i, v := range vals {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteByte('"')
		b.WriteString(strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(v))
		b.WriteByte('"')
	}
	b.WriteByte('}')
	return b.String()
}
