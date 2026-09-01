package svc

import (
	"github.com/menli02/QR-menu/services/catalog/client/catalogservice"
	"github.com/menli02/QR-menu/services/order/internal/config"
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
}

func NewServiceContext(c config.Config) *ServiceContext {
	return &ServiceContext{
		Config:     c,
		DB:         postgres.New(c.Postgres.DataSource),
		CatalogRpc: catalogservice.NewCatalogService(zrpc.MustNewClient(c.CatalogRpc)),
	}
}
