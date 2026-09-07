// Package venue gives the order service a cached, typed view of the
// per-venue settings that live in catalog_db.
//
// Order can't read catalog's database (docs/TZ.md §6: no cross-service
// foreign keys, and no cross-service DB access either), and several of
// FR-O5's submit-time rules — the comment length ceiling, the order total
// limit — are venue settings. ResolveOrderItems does not return them, so
// they need their own lookup.
//
// Doing that lookup on every CreateOrder would add a second synchronous
// RPC to the order path for data that changes a few times a year, so it
// is cached with a short TTL. The consequence is explicit and acceptable:
// a settings change can take up to CacheTTL to affect order validation.
// Nothing here is used for pricing — prices always come fresh from
// ResolveOrderItems — so a stale entry can never mis-charge a guest.
package venue

import (
	"context"
	"time"

	"github.com/menli02/QR-menu/proto/catalog/v1"
	"github.com/menli02/QR-menu/services/catalog/client/catalogservice"

	"github.com/zeromicro/go-zero/core/collection"
	"github.com/zeromicro/go-zero/core/logx"
)

// Settings is the subset of catalog's VenueSettings the order service
// actually needs, with the raw proto fields resolved into usable types.
type Settings struct {
	VenueID              string
	Currency             string
	DefaultLocale        string
	Location             *time.Location
	ServiceChargeBps     int32
	CommentMaxLen        int
	OrderTotalLimitMinor int64
	CancelWindow         time.Duration
}

// Defaults applied when a venue leaves a setting at zero. They mirror the
// column defaults in migrations/catalog/000002_schema.up.sql so the two
// layers can't disagree about what "unset" means.
const (
	DefaultCommentMaxLen = 200
	DefaultCancelWindow  = 60 * time.Second
)

// MaxItemsPerOrder is FR-O5's fixed cap. Unlike the limits above it is not
// venue-configurable in R1 — no field carries it — so it lives here rather
// than in Settings.
const MaxItemsPerOrder = 50

type Provider struct {
	catalog catalogservice.CatalogService
	cache   *collection.Cache
}

func NewProvider(catalog catalogservice.CatalogService, ttl time.Duration) (*Provider, error) {
	c, err := collection.NewCache(ttl, collection.WithName("venue-settings"))
	if err != nil {
		return nil, err
	}
	return &Provider{catalog: catalog, cache: c}, nil
}

// Get returns venueID's settings, fetching them from catalog on a miss.
// collection.Cache.Take collapses concurrent misses for the same venue
// into one RPC, so a burst of orders at open time doesn't stampede
// catalog.
func (p *Provider) Get(ctx context.Context, venueID string) (*Settings, error) {
	v, err := p.cache.Take(venueID, func() (any, error) {
		resp, err := p.catalog.GetVenueSettings(ctx, &v1_catalogpb.GetVenueSettingsRequest{VenueId: venueID})
		if err != nil {
			return nil, err
		}
		return fromProto(resp), nil
	})
	if err != nil {
		return nil, err
	}
	return v.(*Settings), nil
}

func fromProto(s *v1_catalogpb.VenueSettings) *Settings {
	out := &Settings{
		VenueID:              s.GetVenueId(),
		Currency:             s.GetCurrency(),
		DefaultLocale:        s.GetDefaultLocale(),
		Location:             loadLocation(s.GetTimezone()),
		ServiceChargeBps:     s.GetServiceChargeBps(),
		CommentMaxLen:        int(s.GetOrderItemCommentMaxLen()),
		OrderTotalLimitMinor: s.GetOrderTotalLimitMinor(),
		CancelWindow:         time.Duration(s.GetCancelWindowSeconds()) * time.Second,
	}
	if out.CommentMaxLen <= 0 {
		out.CommentMaxLen = DefaultCommentMaxLen
	}
	if out.CancelWindow <= 0 {
		out.CancelWindow = DefaultCancelWindow
	}
	return out
}

// loadLocation falls back to UTC on an unparseable timezone rather than
// failing the order. Getting the business day wrong by a few hours is bad;
// refusing to take the order is worse, and the misconfiguration is the
// venue's to fix.
func loadLocation(name string) *time.Location {
	if name == "" {
		return time.UTC
	}
	loc, err := time.LoadLocation(name)
	if err != nil {
		logx.Errorf("venue: unknown timezone %q, falling back to UTC", name)
		return time.UTC
	}
	return loc
}

// BusinessDate is the venue-local business day used for order numbering
// and the day report (FR-O8, FR-A3).
//
// Known gap: FR-A2 lists a configurable "business-day cutoff" among venue
// settings, but neither catalog_db.venues nor VenueSettings carries an
// hour — the schema comment says the timezone alone drives it. So this is
// midnight venue-local. A venue serving past midnight will see the night's
// late orders numbered into the next day. Closing that properly needs an
// additive field (e.g. business_day_cutoff_minute) on VenueSettings; it is
// flagged rather than guessed at here.
func (s *Settings) BusinessDate(t time.Time) time.Time {
	local := t.In(s.Location)
	return time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, time.UTC)
}

// ParseBusinessDate reads a YYYY-MM-DD report parameter into the same
// representation BusinessDate produces (a UTC midnight carrying only the
// date), so the two always compare equal for the same day.
func ParseBusinessDate(s string) (time.Time, error) {
	return time.ParseInLocation("2006-01-02", s, time.UTC)
}
