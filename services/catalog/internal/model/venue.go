package model

import (
	"context"
	"database/sql"
	"time"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

// Venue mirrors catalog_db.venues. There is no Insert here: venue
// provisioning is a platform-operator action out of product scope in R1
// (docs/TZ.md §4.1) — every row is expected to already exist by the time
// this service reads or updates it.
type Venue struct {
	ID                       string         `db:"id"`
	Slug                     string         `db:"slug"`
	Name                     string         `db:"name"`
	LogoURL                  sql.NullString `db:"logo_url"`
	Currency                 string         `db:"currency"`
	Locales                  stringSlice    `db:"locales"`
	DefaultLocale            string         `db:"default_locale"`
	Timezone                 string         `db:"timezone"`
	ServiceChargeBps         int32          `db:"service_charge_bps"`
	OrderItemCommentMaxLen   int32          `db:"order_item_comment_max_len"`
	OrderTotalLimitMinor     sql.NullInt64  `db:"order_total_limit_minor"`
	CancelWindowSeconds      int32          `db:"cancel_window_seconds"`
	KDSAmberThresholdSeconds int32          `db:"kds_amber_threshold_seconds"`
	KDSRedThresholdSeconds   int32          `db:"kds_red_threshold_seconds"`
	BusinessDayCutoffMinute  int32          `db:"business_day_cutoff_minute"`
	MenuVersion              int64          `db:"menu_version"`
	CreatedAt                time.Time      `db:"created_at"`
	UpdatedAt                time.Time      `db:"updated_at"`
}

// VenueModel, like every model in this package, is bound to a
// sqlx.Session rather than a sqlx.SqlConn: many catalog writes need to
// touch several tables atomically (e.g. insert a menu item, insert its
// availability windows, and bump venues.menu_version, all in one
// transaction), so the logic layer opens a transaction via
// svcCtx.DB.TransactCtx and constructs short-lived model instances bound
// to that transaction's Session — a plain sqlx.SqlConn (e.g. svcCtx.DB
// itself) satisfies Session equally well for calls that don't need to
// join a wider transaction.
type VenueModel struct {
	conn sqlx.Session
}

func NewVenueModel(conn sqlx.Session) *VenueModel {
	return &VenueModel{conn: conn}
}

const venueCols = `id, slug, name, logo_url, currency, locales, default_locale, timezone,
	service_charge_bps, order_item_comment_max_len, order_total_limit_minor,
	cancel_window_seconds, kds_amber_threshold_seconds, kds_red_threshold_seconds,
	business_day_cutoff_minute, menu_version, created_at, updated_at`

func (m *VenueModel) FindByID(ctx context.Context, id string) (*Venue, error) {
	var v Venue
	err := m.conn.QueryRowCtx(ctx, &v, `SELECT `+venueCols+` FROM venues WHERE id = $1`, id)
	if err != nil {
		return nil, err
	}
	return &v, nil
}

// FindBySlug is ResolveTable's entry point: the QR link carries venue_slug,
// not venue_id (catalog.proto ResolveTableRequest).
func (m *VenueModel) FindBySlug(ctx context.Context, slug string) (*Venue, error) {
	var v Venue
	err := m.conn.QueryRowCtx(ctx, &v, `SELECT `+venueCols+` FROM venues WHERE slug = $1`, slug)
	if err != nil {
		return nil, err
	}
	return &v, nil
}

// UpdateSettings changes venue-level configuration (FR-A2). It never
// touches menu_version: none of these fields affect what menu content is
// shown, only how orders/tickets around it behave.
func (m *VenueModel) UpdateSettings(ctx context.Context, v *Venue) (*Venue, error) {
	var out Venue
	err := m.conn.QueryRowCtx(ctx, &out, `
		UPDATE venues SET
			name = $2,
			logo_url = $3,
			currency = $4,
			locales = $5::text[],
			default_locale = $6,
			timezone = $7,
			service_charge_bps = $8,
			order_item_comment_max_len = $9,
			order_total_limit_minor = $10,
			cancel_window_seconds = $11,
			kds_amber_threshold_seconds = $12,
			kds_red_threshold_seconds = $13,
			business_day_cutoff_minute = $14
		WHERE id = $1
		RETURNING `+venueCols,
		v.ID, v.Name, v.LogoURL, v.Currency, pgTextArrayLiteral(v.Locales), v.DefaultLocale, v.Timezone,
		v.ServiceChargeBps, v.OrderItemCommentMaxLen, v.OrderTotalLimitMinor,
		v.CancelWindowSeconds, v.KDSAmberThresholdSeconds, v.KDSRedThresholdSeconds,
		v.BusinessDayCutoffMinute)
	if err != nil {
		return nil, err
	}
	return &out, nil
}

// BumpMenuVersion increments venues.menu_version and returns the new
// value. Called in the same transaction as any write that changes what
// the guest menu shows (category/item/modifier CRUD, availability
// toggles) — see this column's comment in the migration for why it's an
// explicit call here rather than a DB trigger.
func (m *VenueModel) BumpMenuVersion(ctx context.Context, venueID string) (int64, error) {
	var newVersion int64
	err := m.conn.QueryRowCtx(ctx, &newVersion,
		`UPDATE venues SET menu_version = menu_version + 1 WHERE id = $1 RETURNING menu_version`, venueID)
	return newVersion, err
}
