package config

import "github.com/zeromicro/go-zero/zrpc"

type Config struct {
	zrpc.RpcServerConf

	// Postgres is the order_db connection: table/guest sessions, orders,
	// order items, service requests, bill, idempotency_key, outbox
	// (docs/TZ.md §6, §7.4). Local dev default matches
	// deploy/docker-compose.yml; production values are injected from a
	// Kubernetes Secret, never committed here.
	Postgres struct {
		DataSource string
	}

	// CatalogRpc is the etcd-discovered client used to price and validate
	// items on CreateOrder (docs/TZ.md §7.2: order -> catalog.ResolveOrderItems).
	CatalogRpc zrpc.RpcClientConf

	// KafkaBrokers feeds the transactional outbox relay that publishes
	// qrmenu.order.v1 and qrmenu.service_request.v1 events
	// (docs/TZ.md §7.2, §8.3). Empty disables the relay, which is the
	// right local-dev default: outbox rows still accumulate and can be
	// inspected, nothing tries to reach a broker that isn't there.
	KafkaBrokers []string

	// Outbox tunes the relay goroutine. Defaults are set in NewRelay when
	// a field is left at zero.
	Outbox struct {
		PollIntervalMs int
		BatchSize      int
		MaxAttempts    int
	}

	// GuestSessionTTLSeconds must match identity's guest token TTL: this
	// service records expires_at on guest_sessions as a bookkeeping copy
	// of the JWT's own `exp` (see the migration's comment). Defaults to
	// 4h (FR-O1) when unset.
	GuestSessionTTLSeconds int

	// ServiceRequestTTLSeconds is FR-S4's auto-expiry window for an open
	// call-waiter / request-bill. Defaults to 15 min when unset.
	ServiceRequestTTLSeconds int

	// VenueSettingsCacheSeconds bounds how stale the cached copy of a
	// venue's order limits may be (see internal/venue). Defaults to 60s.
	VenueSettingsCacheSeconds int
}

// Effective values, with the documented defaults applied. Kept as methods
// rather than baked into the yaml so an omitted key and an explicit zero
// behave identically and the default lives next to the field it belongs to.
const (
	defaultGuestSessionTTLSeconds    = 4 * 60 * 60
	defaultServiceRequestTTLSeconds  = 15 * 60
	defaultVenueSettingsCacheSeconds = 60
)

func (c Config) GuestSessionTTLOrDefault() int {
	if c.GuestSessionTTLSeconds > 0 {
		return c.GuestSessionTTLSeconds
	}
	return defaultGuestSessionTTLSeconds
}

func (c Config) ServiceRequestTTLOrDefault() int {
	if c.ServiceRequestTTLSeconds > 0 {
		return c.ServiceRequestTTLSeconds
	}
	return defaultServiceRequestTTLSeconds
}

func (c Config) VenueSettingsCacheOrDefault() int {
	if c.VenueSettingsCacheSeconds > 0 {
		return c.VenueSettingsCacheSeconds
	}
	return defaultVenueSettingsCacheSeconds
}
