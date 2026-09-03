package model

import (
	"context"
	"time"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

// Table mirrors catalog_db.tables.
type Table struct {
	ID         string    `db:"id"`
	VenueID    string    `db:"venue_id"`
	HallID     string    `db:"hall_id"`
	Label      string    `db:"label"`
	Seats      int32     `db:"seats"`
	IsActive   bool      `db:"is_active"`
	TableCode  string    `db:"table_code"`
	KeyVersion int32     `db:"key_version"`
	CreatedAt  time.Time `db:"created_at"`
	UpdatedAt  time.Time `db:"updated_at"`
}

type TableModel struct {
	conn sqlx.Session
}

func NewTableModel(conn sqlx.Session) *TableModel {
	return &TableModel{conn: conn}
}

const tableCols = "id, venue_id, hall_id, label, seats, is_active, table_code, key_version, created_at, updated_at"

// Insert assigns tableCode and keyVersion once, at creation — both are
// immutable afterward (FR-T2). Reprinting a table under a newer key after
// rotation isn't wired up anywhere yet (there's no QR export endpoint to
// reprint from — gateway's /admin/tables/qr.pdf is still a 501 stub), so
// Update below deliberately never touches either field.
func (m *TableModel) Insert(ctx context.Context, venueID, hallID, label string, seats int32, tableCode string, keyVersion int32) (*Table, error) {
	var t Table
	err := m.conn.QueryRowCtx(ctx, &t, `
		INSERT INTO tables (venue_id, hall_id, label, seats, table_code, key_version)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING `+tableCols,
		venueID, hallID, label, seats, tableCode, keyVersion)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func (m *TableModel) FindByID(ctx context.Context, venueID, id string) (*Table, error) {
	var t Table
	err := m.conn.QueryRowCtx(ctx, &t, `SELECT `+tableCols+` FROM tables WHERE id = $1 AND venue_id = $2`, id, venueID)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

// FindByVenueAndCode is ResolveTable's lookup (docs/TZ.md FR-T2): a QR
// link names a venue_slug + table_code, never a table id directly.
func (m *TableModel) FindByVenueAndCode(ctx context.Context, venueID, tableCode string) (*Table, error) {
	var t Table
	err := m.conn.QueryRowCtx(ctx, &t,
		`SELECT `+tableCols+` FROM tables WHERE venue_id = $1 AND table_code = $2`, venueID, tableCode)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

// List is keyset-paginated (ListTablesRequest.cursor/page_size), with an
// optional hall filter.
func (m *TableModel) List(ctx context.Context, venueID, hallID string, after Cursor, limit int) ([]Table, error) {
	var rows []Table
	var err error
	if hallID == "" {
		err = m.conn.QueryRowsCtx(ctx, &rows, `
			SELECT `+tableCols+` FROM tables
			WHERE venue_id = $1 AND (created_at, id) > ($2, $3)
			ORDER BY created_at, id
			LIMIT $4`,
			venueID, after.CreatedAt, after.idOrZero(), limit)
	} else {
		err = m.conn.QueryRowsCtx(ctx, &rows, `
			SELECT `+tableCols+` FROM tables
			WHERE venue_id = $1 AND hall_id = $2 AND (created_at, id) > ($3, $4)
			ORDER BY created_at, id
			LIMIT $5`,
			venueID, hallID, after.CreatedAt, after.idOrZero(), limit)
	}
	if err != nil {
		return nil, err
	}
	return rows, nil
}

// Update changes label/hall/seats/is_active only — see Insert's comment
// for why table_code and key_version are never touched here.
func (m *TableModel) Update(ctx context.Context, id, venueID, hallID, label string, seats int32, isActive bool) (*Table, error) {
	var t Table
	err := m.conn.QueryRowCtx(ctx, &t, `
		UPDATE tables SET hall_id = $3, label = $4, seats = $5, is_active = $6
		WHERE id = $1 AND venue_id = $2
		RETURNING `+tableCols,
		id, venueID, hallID, label, seats, isActive)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

// Deactivate implements FR-T5 exactly: "Deactivating a table blocks new
// orders with a clear message and keeps history."
func (m *TableModel) Deactivate(ctx context.Context, id, venueID string) (bool, error) {
	res, err := m.conn.ExecCtx(ctx, `UPDATE tables SET is_active = false WHERE id = $1 AND venue_id = $2`, id, venueID)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	return n > 0, err
}
