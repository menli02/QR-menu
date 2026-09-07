package svc

import (
	"time"

	"github.com/menli02/QR-menu/services/catalog/client/catalogservice"
	"github.com/menli02/QR-menu/services/gateway/internal/config"
	"github.com/menli02/QR-menu/services/gateway/internal/middleware"
	"github.com/menli02/QR-menu/services/gateway/internal/ws"
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

	// Hub and WS are the realtime fan-out (docs/TZ.md §7.3). The hub is
	// shared with the Kafka consumer, which is what connects an event
	// published by a service to a socket held by this instance.
	Hub *ws.Hub
	WS  *ws.Handler
}

func NewServiceContext(c config.Config) *ServiceContext {
	// Built once, upfront, so both auth middlewares and the IdentityRpc
	// field below share the same underlying client/connection.
	identityRpc := identityservice.NewIdentityService(zrpc.MustNewClient(c.IdentityRpc))
	orderRpc := orderservice.NewOrderService(zrpc.MustNewClient(c.OrderRpc))
	jwksRefreshInterval := time.Duration(c.JWKSRefreshSeconds) * time.Second

	// The middleware structs, not just their Handle methods: the
	// WebSocket endpoints reuse their Verify to check the token that
	// arrives in the first frame, sharing one JWKS cache with the REST
	// routes rather than keeping a second one warm.
	staffAuth := middleware.NewStaffAuthMiddleware(identityRpc, jwksRefreshInterval)
	guestAuth := middleware.NewGuestAuthMiddleware(identityRpc, jwksRefreshInterval)

	hub := ws.NewHub()

	return &ServiceContext{
		Config:    c,
		StaffAuth: staffAuth.Handle,
		GuestAuth: guestAuth.Handle,

		CatalogRpc:  catalogservice.NewCatalogService(zrpc.MustNewClient(c.CatalogRpc)),
		OrderRpc:    orderRpc,
		IdentityRpc: identityRpc,

		Redis: redis.MustNewRedis(c.Redis),

		Hub: hub,
		WS: ws.NewHandler(
			hub,
			staffVerifier{m: staffAuth},
			guestVerifier{m: guestAuth},
			sessionResolver{order: orderRpc},
			c.WSAllowedOrigins,
		),
	}
}
