package svc

import (
	"time"

	"github.com/menli02/QR-menu/services/catalog/client/catalogservice"
	"github.com/menli02/QR-menu/services/order/internal/config"
	"github.com/menli02/QR-menu/services/order/internal/venue"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/postgres"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"github.com/zeromicro/go-zero/zrpc"
)

type ServiceContext struct {
	Config config.Config

	// DB is a lazy connection pool: dialing happens on first query, so a
	// transient Postgres outage at boot never crash-loops this service —
	// readiness should be reported by a DB ping, not by startup success.
	DB sqlx.SqlConn

	// CatalogRpc resolves prices/availability for requested order items
	// before a CreateOrder transaction commits (docs/TZ.md §7.2).
	CatalogRpc catalogservice.CatalogService

	// Venue caches the per-venue limits FR-O5 validates against; see
	// internal/venue for why they aren't read per request.
	Venue *venue.Provider

	// GuestSessionTTL and ServiceRequestTTL are resolved once here so
	// logic files take a duration rather than re-deriving it from config.
	GuestSessionTTL   time.Duration
	ServiceRequestTTL time.Duration
}

func NewServiceContext(c config.Config) *ServiceContext {
	catalog := catalogservice.NewCatalogService(zrpc.MustNewClient(c.CatalogRpc))

	venues, err := venue.NewProvider(catalog, time.Duration(c.VenueSettingsCacheOrDefault())*time.Second)
	logx.Must(err)

	return &ServiceContext{
		Config:            c,
		DB:                postgres.New(c.Postgres.DataSource),
		CatalogRpc:        catalog,
		Venue:             venues,
		GuestSessionTTL:   time.Duration(c.GuestSessionTTLOrDefault()) * time.Second,
		ServiceRequestTTL: time.Duration(c.ServiceRequestTTLOrDefault()) * time.Second,
	}
}
