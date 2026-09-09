package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"

	"github.com/menli02/QR-menu/pkg/health"
	"github.com/menli02/QR-menu/services/gateway/internal/config"
	"github.com/menli02/QR-menu/services/gateway/internal/consumer"
	"github.com/menli02/QR-menu/services/gateway/internal/errs"
	"github.com/menli02/QR-menu/services/gateway/internal/handler"
	adminhandler "github.com/menli02/QR-menu/services/gateway/internal/handler/admin"
	"github.com/menli02/QR-menu/services/gateway/internal/reqctx"
	"github.com/menli02/QR-menu/services/gateway/internal/svc"

	"github.com/zeromicro/go-zero/core/conf"
	"github.com/zeromicro/go-zero/core/service"
	"github.com/zeromicro/go-zero/rest"
	"github.com/zeromicro/go-zero/rest/httpx"
	"go.opentelemetry.io/otel/trace"
)

var configFile = flag.String("f", "etc/gateway-api.yaml", "the config file")

func main() {
	flag.Parse()

	var c config.Config
	conf.MustLoad(*configFile, &c)

	httpx.SetErrorHandlerCtx(errorHandler)

	server := rest.MustNewServer(c.RestConf)
	defer server.Stop()

	// Global, before routing: lifts Idempotency-Key and the client IP into
	// the request context. goctl's logic signatures receive no
	// *http.Request, so this is how a header reaches a handler without
	// hand-editing generated code that regeneration would overwrite.
	server.Use(reqctx.Middleware)

	ctx := svc.NewServiceContext(c)
	handler.RegisterHandlers(server, ctx)
	registerUnroutedRoutes(server, ctx)

	// The Kafka consumer and the REST server share a hub: an event
	// published by order or catalog reaches the sockets this instance
	// holds (docs/TZ.md §7.3). Every gateway pod uses its own consumer
	// group, so all of them see every event — see the consumer package.
	realtime := consumer.New(ctx.Hub, consumer.Config{
		Brokers:     c.KafkaBrokers,
		GroupPrefix: c.KafkaGroupPrefix,
		InstanceID:  c.KafkaInstanceID,
	})

	// The gateway owns no database; what it cannot start usefully without
	// is its downstream services, and those are discovered through etcd.
	// See pkg/health for why this drives the startup probe only.
	ready := health.NewServer(c.ReadinessPort)
	if hosts := c.IdentityRpc.Etcd.Hosts; len(hosts) > 0 {
		// One endpoint is enough: this asks whether service discovery is
		// reachable at all, which is what a freshly scheduled pod needs to
		// prove. Checking every member would turn a single etcd node being
		// down — which the cluster tolerates by design — into a failed
		// rollout.
		ready.Register("etcd", health.TCPCheck(hosts[0]))
	}

	group := service.NewServiceGroup()
	defer group.Stop()
	group.Add(server)
	group.Add(realtime)
	group.Add(ready)

	fmt.Printf("Starting server at %s:%d...\n", c.Host, c.Port)
	group.Start()
}

// errorHandler implements the public error envelope (docs/TZ.md §8.1):
// any error a logic function returns that isn't an *errs.Error is reported
// as an opaque 500 INTERNAL — never leak internal error text to clients.
func errorHandler(ctx context.Context, err error) (int, any) {
	var body errs.Body
	if ce, ok := err.(*errs.Error); ok {
		body.Error.Code = ce.Code
		body.Error.Message = ce.Message
		body.Error.Details = ce.Details
		body.Error.TraceID = traceID(ctx)
		return errs.HTTPStatus(ce.Code), body
	}

	body.Error.Code = errs.CodeInternal
	body.Error.Message = "internal error"
	body.Error.TraceID = traceID(ctx)
	return errs.HTTPStatus(errs.CodeInternal), body
}

func traceID(ctx context.Context) string {
	if id := trace.SpanContextFromContext(ctx).TraceID(); id.IsValid() {
		return id.String()
	}
	return ""
}

// registerUnroutedRoutes wires the contract routes that don't fit the
// goctl .api JSON DSL: WebSocket upgrades and a binary PDF download.
//
// The sockets authenticate from their first frame rather than an
// Authorization header (docs/TZ.md §8.1), so they carry no auth middleware
// here — the handler verifies the token itself, sharing the middlewares'
// JWKS cache.
func registerUnroutedRoutes(server *rest.Server, ctx *svc.ServiceContext) {
	server.AddRoute(rest.Route{Method: http.MethodGet, Path: "/ws/guest", Handler: ctx.WS.ServeGuest})
	server.AddRoute(rest.Route{Method: http.MethodGet, Path: "/ws/staff", Handler: ctx.WS.ServeStaff})

	// Was 501 until catalog grew GetTablePrintCodes: the blocker was never
	// the rendering, it was that nothing in the contract exposed a table's
	// HMAC signature and the gateway cannot derive one.
	// StaffAuth here, and authz.Admin inside the handler: the middleware
	// establishes who the caller is, the handler decides whether they may
	// export a venue's QR credentials. Same split as every generated
	// /admin route.
	server.AddRoutes(rest.WithMiddleware(ctx.StaffAuth, rest.Route{
		Method:  http.MethodGet,
		Path:    "/api/v1/admin/tables/qr.pdf",
		Handler: adminhandler.QRPDFHandler(ctx),
	}))
}
