package svc

import (
	"github.com/menli02/QR-menu/services/identity/internal/config"
	"github.com/zeromicro/go-zero/core/stores/postgres"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

type ServiceContext struct {
	Config config.Config

	// DB is a lazy connection pool: dialing happens on first query, so a
	// transient Postgres outage at boot never crash-loops this service —
	// readiness should be reported by a DB ping, not by startup success.
	DB sqlx.SqlConn
}

func NewServiceContext(c config.Config) *ServiceContext {
	return &ServiceContext{
		Config: c,
		DB:     postgres.New(c.Postgres.DataSource),
	}
}
