package model

import (
	"context"
	"time"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

// MenuItem mirrors catalog_db.menu_items. No currency field: price is
// always in the owning venue's currency (A1) — see the migration's
// comment on why storing one per-item would let it silently diverge.
type MenuItem struct {
	ID             string        `db:"id"`
	VenueID        string        `db:"venue_id"`
	CategoryID     string        `db:"category_id"`
	Name           localizedText `db:"name"`
	Description    localizedText `db:"description"`
	BasePriceMinor int64         `db:"base_price_minor"`
	ImageURL       string        `db:"image_url"`
	Allergens      stringSlice   `db:"allergens"`
	IsActive       bool          `db:"is_active"`    // soft-deleted/published flag
	IsAvailable    bool          `db:"is_available"` // stop-list state (FR-C4)
	SortOrder      int32         `db:"sort_order"`
	CreatedAt      time.Time     `db:"created_at"`
	UpdatedAt      time.Time     `db:"updated_at"`
}

type MenuItemModel struct {
	conn sqlx.Session
}

func NewMenuItemModel(conn sqlx.Session) *MenuItemModel {
	return &MenuItemModel{conn: conn}
}

const menuItemCols = `id, venue_id, category_id, name, description, base_price_minor, image_url,
	allergens, is_active, is_available, sort_order, created_at, updated_at`

func (m *MenuItemModel) Insert(ctx context.Context, venueID, categoryID string, name, description localizedText, basePriceMinor int64, imageURL string, allergens []string, sortOrder int32) (*MenuItem, error) {
	var item MenuItem
	err := m.conn.QueryRowCtx(ctx, &item, `
		INSERT INTO menu_items (venue_id, category_id, name, description, base_price_minor, image_url, allergens, sort_order)
		VALUES ($1, $2, $3, $4, $5, $6, $7::text[], $8)
		RETURNING `+menuItemCols,
		venueID, categoryID, name, description, basePriceMinor, imageURL, pgTextArrayLiteral(allergens), sortOrder)
	if err != nil {
		return nil, err
	}
	return &item, nil
}

func (m *MenuItemModel) FindByID(ctx context.Context, venueID, id string) (*MenuItem, error) {
	var item MenuItem
	err := m.conn.QueryRowCtx(ctx, &item, `SELECT `+menuItemCols+` FROM menu_items WHERE id = $1 AND venue_id = $2`, id, venueID)
	if err != nil {
		return nil, err
	}
	return &item, nil
}

// FindByIDs is ResolveOrderItems's batch lookup: one query for every
// requested item_id, rather than one round trip per line of the cart.
// Missing ids simply aren't in the result — the caller (ResolveOrderItems
// logic) is responsible for noticing and reporting them as NOT_FOUND.
func (m *MenuItemModel) FindByIDs(ctx context.Context, venueID string, ids []string) ([]MenuItem, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	var rows []MenuItem
	err := m.conn.QueryRowsCtx(ctx, &rows,
		`SELECT `+menuItemCols+` FROM menu_items WHERE venue_id = $1 AND id = ANY($2::uuid[])`,
		venueID, pgTextArrayLiteral(ids))
	if err != nil {
		return nil, err
	}
	return rows, nil
}

// ListByVenue is the admin listing (FR-C2): every item regardless of
// active/available state, optionally filtered to one category, keyset
// paginated.
func (m *MenuItemModel) ListByVenue(ctx context.Context, venueID, categoryID string, after Cursor, limit int) ([]MenuItem, error) {
	var rows []MenuItem
	var err error
	if categoryID == "" {
		err = m.conn.QueryRowsCtx(ctx, &rows, `
			SELECT `+menuItemCols+` FROM menu_items
			WHERE venue_id = $1 AND (created_at, id) > ($2, $3)
			ORDER BY created_at, id
			LIMIT $4`,
			venueID, after.CreatedAt, after.idOrZero(), limit)
	} else {
		err = m.conn.QueryRowsCtx(ctx, &rows, `
			SELECT `+menuItemCols+` FROM menu_items
			WHERE venue_id = $1 AND category_id = $2 AND (created_at, id) > ($3, $4)
			ORDER BY created_at, id
			LIMIT $5`,
			venueID, categoryID, after.CreatedAt, after.idOrZero(), limit)
	}
	if err != nil {
		return nil, err
	}
	return rows, nil
}

// ListForMenu is GetMenu's entry point (FR-C5): only active, available
// items, ordered the way a guest should see them.
func (m *MenuItemModel) ListForMenu(ctx context.Context, venueID string) ([]MenuItem, error) {
	var rows []MenuItem
	err := m.conn.QueryRowsCtx(ctx, &rows, `
		SELECT `+menuItemCols+` FROM menu_items
		WHERE venue_id = $1 AND is_active = true AND is_available = true
		ORDER BY category_id, sort_order, id`,
		venueID)
	if err != nil {
		return nil, err
	}
	return rows, nil
}

// Update changes catalog/content fields only — never is_active
// (Deactivate) or is_available (SetAvailability), which have their own
// narrower, more auditable entry points.
//
// name and description are *merged* into the stored translation maps
// (JSONB `||`), not replaced — see jsonb.go for the rationale.
func (m *MenuItemModel) Update(ctx context.Context, id, venueID, categoryID string, name, description localizedText, basePriceMinor int64, imageURL string, allergens []string, sortOrder int32) (*MenuItem, error) {
	var item MenuItem
	err := m.conn.QueryRowCtx(ctx, &item, `
		UPDATE menu_items SET
			category_id = $3, name = name || $4::jsonb, description = description || $5::jsonb,
			base_price_minor = $6, image_url = $7, allergens = $8::text[], sort_order = $9
		WHERE id = $1 AND venue_id = $2
		RETURNING `+menuItemCols,
		id, venueID, categoryID, name, description, basePriceMinor, imageURL, pgTextArrayLiteral(allergens), sortOrder)
	if err != nil {
		return nil, err
	}
	return &item, nil
}

// Deactivate soft-deletes (is_active = false) — the proto field comment
// on MenuItem.is_active literally says "soft-deleted/published flag".
func (m *MenuItemModel) Deactivate(ctx context.Context, id, venueID string) (bool, error) {
	res, err := m.conn.ExecCtx(ctx, `UPDATE menu_items SET is_active = false WHERE id = $1 AND venue_id = $2`, id, venueID)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	return n > 0, err
}

// SetAvailability toggles the stop-list state (FR-C4, FR-K6).
func (m *MenuItemModel) SetAvailability(ctx context.Context, id, venueID string, isAvailable bool) (*MenuItem, error) {
	var item MenuItem
	err := m.conn.QueryRowCtx(ctx, &item, `
		UPDATE menu_items SET is_available = $3
		WHERE id = $1 AND venue_id = $2
		RETURNING `+menuItemCols,
		id, venueID, isAvailable)
	if err != nil {
		return nil, err
	}
	return &item, nil
}

// ---------------------------------------------------------------------
// Availability windows (FR-C7)
// ---------------------------------------------------------------------

// AvailabilityWindow mirrors catalog_db.availability_windows.
type AvailabilityWindow struct {
	ItemID           string `db:"item_id"`
	StartMinuteOfDay int32  `db:"start_minute_of_day"`
	EndMinuteOfDay   int32  `db:"end_minute_of_day"`
}

type AvailabilityWindowModel struct {
	conn sqlx.Session
}

func NewAvailabilityWindowModel(conn sqlx.Session) *AvailabilityWindowModel {
	return &AvailabilityWindowModel{conn: conn}
}

// ReplaceForItem swaps itemID's whole window set — CreateMenuItem/
// UpdateMenuItem always send the complete list (proto has no "add one
// window" RPC), so delete-then-insert is simpler and just as correct as
// diffing.
func (m *AvailabilityWindowModel) ReplaceForItem(ctx context.Context, itemID string, windows []AvailabilityWindow) error {
	if _, err := m.conn.ExecCtx(ctx, `DELETE FROM availability_windows WHERE item_id = $1`, itemID); err != nil {
		return err
	}
	for _, w := range windows {
		if _, err := m.conn.ExecCtx(ctx, `
			INSERT INTO availability_windows (item_id, start_minute_of_day, end_minute_of_day)
			VALUES ($1, $2, $3)`,
			itemID, w.StartMinuteOfDay, w.EndMinuteOfDay); err != nil {
			return err
		}
	}
	return nil
}

// ListByItemIDs batches the fan-out for GetMenu/ListMenuItems assembly:
// one query for every item on the page, not one per item.
func (m *AvailabilityWindowModel) ListByItemIDs(ctx context.Context, itemIDs []string) ([]AvailabilityWindow, error) {
	if len(itemIDs) == 0 {
		return nil, nil
	}
	var rows []AvailabilityWindow
	err := m.conn.QueryRowsCtx(ctx, &rows,
		`SELECT item_id, start_minute_of_day, end_minute_of_day FROM availability_windows WHERE item_id = ANY($1::uuid[])`,
		pgTextArrayLiteral(itemIDs))
	if err != nil {
		return nil, err
	}
	return rows, nil
}
