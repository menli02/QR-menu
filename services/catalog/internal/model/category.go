package model

import (
	"context"
	"time"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

// Category mirrors catalog_db.categories.
type Category struct {
	ID        string        `db:"id"`
	VenueID   string        `db:"venue_id"`
	Name      localizedText `db:"name"`
	SortOrder int32         `db:"sort_order"`
	IsVisible bool          `db:"is_visible"`
	ImageURL  string        `db:"image_url"`
	CreatedAt time.Time     `db:"created_at"`
	UpdatedAt time.Time     `db:"updated_at"`
}

type CategoryModel struct {
	conn sqlx.Session
}

func NewCategoryModel(conn sqlx.Session) *CategoryModel {
	return &CategoryModel{conn: conn}
}

const categoryCols = "id, venue_id, name, sort_order, is_visible, image_url, created_at, updated_at"

func (m *CategoryModel) Insert(ctx context.Context, venueID string, name localizedText, sortOrder int32, isVisible bool, imageURL string) (*Category, error) {
	var c Category
	err := m.conn.QueryRowCtx(ctx, &c, `
		INSERT INTO categories (venue_id, name, sort_order, is_visible, image_url)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING `+categoryCols,
		venueID, name, sortOrder, isVisible, imageURL)
	if err != nil {
		return nil, err
	}
	return &c, nil
}

func (m *CategoryModel) FindByID(ctx context.Context, venueID, id string) (*Category, error) {
	var c Category
	err := m.conn.QueryRowCtx(ctx, &c, `SELECT `+categoryCols+` FROM categories WHERE id = $1 AND venue_id = $2`, id, venueID)
	if err != nil {
		return nil, err
	}
	return &c, nil
}

// List returns every category for venueID — ListCategoriesResponse isn't
// cursor-paginated (proto/catalog/v1/catalog.proto).
func (m *CategoryModel) List(ctx context.Context, venueID string) ([]Category, error) {
	var rows []Category
	err := m.conn.QueryRowsCtx(ctx, &rows,
		`SELECT `+categoryCols+` FROM categories WHERE venue_id = $1 ORDER BY sort_order, id`, venueID)
	if err != nil {
		return nil, err
	}
	return rows, nil
}

// ListVisible is GetMenu's entry point (FR-C5): only categories a guest
// should see, in display order.
func (m *CategoryModel) ListVisible(ctx context.Context, venueID string) ([]Category, error) {
	var rows []Category
	err := m.conn.QueryRowsCtx(ctx, &rows,
		`SELECT `+categoryCols+` FROM categories WHERE venue_id = $1 AND is_visible = true ORDER BY sort_order, id`, venueID)
	if err != nil {
		return nil, err
	}
	return rows, nil
}

// Update merges `name` into the stored translation map (JSONB `||`)
// rather than replacing it — see the note on localizedText merge semantics
// in jsonb.go for why, and for what that costs.
func (m *CategoryModel) Update(ctx context.Context, id, venueID string, name localizedText, sortOrder int32, isVisible bool, imageURL string) (*Category, error) {
	var c Category
	err := m.conn.QueryRowCtx(ctx, &c, `
		UPDATE categories SET name = name || $3::jsonb, sort_order = $4, is_visible = $5, image_url = $6
		WHERE id = $1 AND venue_id = $2
		RETURNING `+categoryCols,
		id, venueID, name, sortOrder, isVisible, imageURL)
	if err != nil {
		return nil, err
	}
	return &c, nil
}

// Delete is a genuine hard DELETE, unlike Table/Hall: Category has no
// is_active concept in the domain (proto Category has only is_visible,
// which means "hide from guests", not "retired"). menu_items.category_id
// has no ON DELETE CASCADE, so deleting a category that still has items
// fails the foreign key — surfaced by the caller as FAILED_PRECONDITION,
// not silently ignored or cascaded.
func (m *CategoryModel) Delete(ctx context.Context, id, venueID string) (bool, error) {
	res, err := m.conn.ExecCtx(ctx, `DELETE FROM categories WHERE id = $1 AND venue_id = $2`, id, venueID)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	return n > 0, err
}
