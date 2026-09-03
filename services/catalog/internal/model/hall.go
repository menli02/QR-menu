package model

import (
	"context"
	"time"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

// Hall mirrors catalog_db.halls.
type Hall struct {
	ID        string    `db:"id"`
	VenueID   string    `db:"venue_id"`
	Name      string    `db:"name"`
	SortOrder int32     `db:"sort_order"`
	IsActive  bool      `db:"is_active"`
	CreatedAt time.Time `db:"created_at"`
	UpdatedAt time.Time `db:"updated_at"`
}

type HallModel struct {
	conn sqlx.Session
}

func NewHallModel(conn sqlx.Session) *HallModel {
	return &HallModel{conn: conn}
}

const hallCols = "id, venue_id, name, sort_order, is_active, created_at, updated_at"

func (m *HallModel) Insert(ctx context.Context, venueID, name string, sortOrder int32) (*Hall, error) {
	var h Hall
	err := m.conn.QueryRowCtx(ctx, &h, `
		INSERT INTO halls (venue_id, name, sort_order)
		VALUES ($1, $2, $3)
		RETURNING `+hallCols,
		venueID, name, sortOrder)
	if err != nil {
		return nil, err
	}
	return &h, nil
}

func (m *HallModel) FindByID(ctx context.Context, venueID, id string) (*Hall, error) {
	var h Hall
	err := m.conn.QueryRowCtx(ctx, &h, `SELECT `+hallCols+` FROM halls WHERE id = $1 AND venue_id = $2`, id, venueID)
	if err != nil {
		return nil, err
	}
	return &h, nil
}

// List returns every hall for venueID — ListHallsResponse isn't
// cursor-paginated (proto/catalog/v1/catalog.proto), and a venue has at
// most a handful of halls.
func (m *HallModel) List(ctx context.Context, venueID string) ([]Hall, error) {
	var rows []Hall
	err := m.conn.QueryRowsCtx(ctx, &rows,
		`SELECT `+hallCols+` FROM halls WHERE venue_id = $1 ORDER BY sort_order, name`, venueID)
	if err != nil {
		return nil, err
	}
	return rows, nil
}

func (m *HallModel) Update(ctx context.Context, id, venueID, name string, sortOrder int32, isActive bool) (*Hall, error) {
	var h Hall
	err := m.conn.QueryRowCtx(ctx, &h, `
		UPDATE halls SET name = $3, sort_order = $4, is_active = $5
		WHERE id = $1 AND venue_id = $2
		RETURNING `+hallCols,
		id, venueID, name, sortOrder, isActive)
	if err != nil {
		return nil, err
	}
	return &h, nil
}

// Deactivate soft-deletes (is_active = false), matching Table's pattern:
// tables.hall_id references this row with no ON DELETE CASCADE, so a
// hard delete of a hall with any tables — active or not — would fail
// that foreign key anyway. Deactivating preserves history and blocks
// nothing that a real DELETE wouldn't already block.
func (m *HallModel) Deactivate(ctx context.Context, id, venueID string) (bool, error) {
	res, err := m.conn.ExecCtx(ctx, `UPDATE halls SET is_active = false WHERE id = $1 AND venue_id = $2`, id, venueID)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	return n > 0, err
}
