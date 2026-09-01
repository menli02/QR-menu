package svc

import (
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
	return &ServiceContext{
		Config:    c,
		StaffAuth: middleware.NewStaffAuthMiddleware().Handle,
		GuestAuth: middleware.NewGuestAuthMiddleware().Handle,

		CatalogRpc:  catalogservice.NewCatalogService(zrpc.MustNewClient(c.CatalogRpc)),
		OrderRpc:    orderservice.NewOrderService(zrpc.MustNewClient(c.OrderRpc)),
		IdentityRpc: identityservice.NewIdentityService(zrpc.MustNewClient(c.IdentityRpc)),

		Redis: redis.MustNewRedis(c.Redis),
	}
}
