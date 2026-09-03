package svc

import (
	"time"

	"github.com/menli02/QR-menu/services/catalog/client/catalogservice"
	"github.com/menli02/QR-menu/services/gateway/internal/config"
	"github.com/menli02/QR-menu/services/gateway/internal/middleware"
	"github.com/menli02/QR-menu/services/identity/client/identityservice"
	"github.com/menli02/QR-menu/services/order/client/orderservice"
	"github.com/zeromicro/go-zero/core/stores/redis"
	"github.com/zeromicro/go-zero/rest"
	"github.com/zeromicro/go-zero/zrpc"
)

type ServiceContext struct {
	Config    config.Config
	StaffAuth rest.Middleware
	GuestAuth rest.Middleware

	CatalogRpc  catalogservice.CatalogService
	OrderRpc    orderservice.OrderService
	IdentityRpc identityservice.IdentityService

	Redis *redis.Redis
}

func NewServiceContext(c config.Config) *ServiceContext {
	// Built once, upfront, so both auth middlewares and the IdentityRpc
	// field below share the same underlying client/connection.
	identityRpc := identityservice.NewIdentityService(zrpc.MustNewClient(c.IdentityRpc))
	jwksRefreshInterval := time.Duration(c.JWKSRefreshSeconds) * time.Second

	return &ServiceContext{
		Config:    c,
		StaffAuth: middleware.NewStaffAuthMiddleware(identityRpc, jwksRefreshInterval).Handle,
		GuestAuth: middleware.NewGuestAuthMiddleware(identityRpc, jwksRefreshInterval).Handle,

		CatalogRpc:  catalogservice.NewCatalogService(zrpc.MustNewClient(c.CatalogRpc)),
		OrderRpc:    orderservice.NewOrderService(zrpc.MustNewClient(c.OrderRpc)),
		IdentityRpc: identityRpc,

		Redis: redis.MustNewRedis(c.Redis),
	}
}
