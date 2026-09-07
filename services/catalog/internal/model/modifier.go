package model

import (
	"context"
	"time"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

// ModifierGroup mirrors catalog_db.modifier_groups. One item owns N
// groups — see that table's migration comment on why this isn't the
// many-to-many the docs/TZ.md §6 ER diagram sketches.
type ModifierGroup struct {
	ID        string        `db:"id"`
	ItemID    string        `db:"item_id"`
	Name      localizedText `db:"name"`
	MinSelect int32         `db:"min_select"`
	MaxSelect int32         `db:"max_select"`
	Required  bool          `db:"required"`
	CreatedAt time.Time     `db:"created_at"`
	UpdatedAt time.Time     `db:"updated_at"`
}

type ModifierGroupModel struct {
	conn sqlx.Session
}

func NewModifierGroupModel(conn sqlx.Session) *ModifierGroupModel {
	return &ModifierGroupModel{conn: conn}
}

const modifierGroupCols = "id, item_id, name, min_select, max_select, required, created_at, updated_at"

func (m *ModifierGroupModel) Insert(ctx context.Context, itemID string, name localizedText, minSelect, maxSelect int32, required bool) (*ModifierGroup, error) {
	var g ModifierGroup
	err := m.conn.QueryRowCtx(ctx, &g, `
		INSERT INTO modifier_groups (item_id, name, min_select, max_select, required)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING `+modifierGroupCols,
		itemID, name, minSelect, maxSelect, required)
	if err != nil {
		return nil, err
	}
	return &g, nil
}

// FindByID is scoped through the owning item, not by group id alone:
// UpdateModifierGroupRequest carries venue_id + a ModifierGroup with
// item_id, and every catalog write is expected to prove it owns what
// it's touching.
func (m *ModifierGroupModel) FindByID(ctx context.Context, itemID, id string) (*ModifierGroup, error) {
	var g ModifierGroup
	err := m.conn.QueryRowCtx(ctx, &g,
		`SELECT `+modifierGroupCols+` FROM modifier_groups WHERE id = $1 AND item_id = $2`, id, itemID)
	if err != nil {
		return nil, err
	}
	return &g, nil
}

// ListByItemIDs batches GetMenu/ListMenuItems assembly's fan-out: one
// query for every item on the page.
func (m *ModifierGroupModel) ListByItemIDs(ctx context.Context, itemIDs []string) ([]ModifierGroup, error) {
	if len(itemIDs) == 0 {
		return nil, nil
	}
	var rows []ModifierGroup
	err := m.conn.QueryRowsCtx(ctx, &rows,
		`SELECT `+modifierGroupCols+` FROM modifier_groups WHERE item_id = ANY($1::uuid[]) ORDER BY item_id, id`,
		pgTextArrayLiteral(itemIDs))
	if err != nil {
		return nil, err
	}
	return rows, nil
}

func (m *ModifierGroupModel) Update(ctx context.Context, id, itemID string, name localizedText, minSelect, maxSelect int32, required bool) (*ModifierGroup, error) {
	var g ModifierGroup
	err := m.conn.QueryRowCtx(ctx, &g, `
		UPDATE modifier_groups SET name = name || $3::jsonb, min_select = $4, max_select = $5, required = $6
		WHERE id = $1 AND item_id = $2
		RETURNING `+modifierGroupCols,
		id, itemID, name, minSelect, maxSelect, required)
	if err != nil {
		return nil, err
	}
	return &g, nil
}

// Delete is a genuine hard DELETE: options cascade (ON DELETE CASCADE),
// and order_item_modifiers snapshots names/prices at order time with no
// FK back to catalog, so historical orders are unaffected either way.
func (m *ModifierGroupModel) Delete(ctx context.Context, id, itemID string) (bool, error) {
	res, err := m.conn.ExecCtx(ctx, `DELETE FROM modifier_groups WHERE id = $1 AND item_id = $2`, id, itemID)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	return n > 0, err
}

// ---------------------------------------------------------------------
// Modifier options
// ---------------------------------------------------------------------

// ModifierOption mirrors catalog_db.modifier_options.
type ModifierOption struct {
	ID              string        `db:"id"`
	GroupID         string        `db:"group_id"`
	Name            localizedText `db:"name"`
	PriceDeltaMinor int64         `db:"price_delta_minor"`
	SortOrder       int32         `db:"sort_order"`
	CreatedAt       time.Time     `db:"created_at"`
	UpdatedAt       time.Time     `db:"updated_at"`
}

type ModifierOptionModel struct {
	conn sqlx.Session
}

func NewModifierOptionModel(conn sqlx.Session) *ModifierOptionModel {
	return &ModifierOptionModel{conn: conn}
}

const modifierOptionCols = "id, group_id, name, price_delta_minor, sort_order, created_at, updated_at"

// ReplaceForGroup swaps groupID's whole option set — Create/
// UpdateModifierGroup always send the complete list (proto has no
// "add one option" RPC), so delete-then-insert is simpler and just as
// correct as diffing.
func (m *ModifierOptionModel) ReplaceForGroup(ctx context.Context, groupID string, options []ModifierOption) ([]ModifierOption, error) {
	if _, err := m.conn.ExecCtx(ctx, `DELETE FROM modifier_options WHERE group_id = $1`, groupID); err != nil {
		return nil, err
	}
	out := make([]ModifierOption, 0, len(options))
	for _, o := range options {
		var inserted ModifierOption
		err := m.conn.QueryRowCtx(ctx, &inserted, `
			INSERT INTO modifier_options (group_id, name, price_delta_minor, sort_order)
			VALUES ($1, $2, $3, $4)
			RETURNING `+modifierOptionCols,
			groupID, o.Name, o.PriceDeltaMinor, o.SortOrder)
		if err != nil {
			return nil, err
		}
		out = append(out, inserted)
	}
	return out, nil
}

// ListByGroupIDs batches GetMenu/ListMenuItems assembly's fan-out.
func (m *ModifierOptionModel) ListByGroupIDs(ctx context.Context, groupIDs []string) ([]ModifierOption, error) {
	if len(groupIDs) == 0 {
		return nil, nil
	}
	var rows []ModifierOption
	err := m.conn.QueryRowsCtx(ctx, &rows,
		`SELECT `+modifierOptionCols+` FROM modifier_options WHERE group_id = ANY($1::uuid[]) ORDER BY group_id, sort_order, id`,
		pgTextArrayLiteral(groupIDs))
	if err != nil {
		return nil, err
	}
	return rows, nil
}

// FindByIDs is ResolveOrderItems's batch lookup for the modifier options
// a cart line references.
func (m *ModifierOptionModel) FindByIDs(ctx context.Context, ids []string) ([]ModifierOption, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	var rows []ModifierOption
	err := m.conn.QueryRowsCtx(ctx, &rows,
		`SELECT `+modifierOptionCols+` FROM modifier_options WHERE id = ANY($1::uuid[])`,
		pgTextArrayLiteral(ids))
	if err != nil {
		return nil, err
	}
	return rows, nil
}
