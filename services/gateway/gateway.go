package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"

	"github.com/menli02/QR-menu/services/gateway/internal/config"
	"github.com/menli02/QR-menu/services/gateway/internal/errs"
	"github.com/menli02/QR-menu/services/gateway/internal/handler"
	"github.com/menli02/QR-menu/services/gateway/internal/svc"

	"github.com/zeromicro/go-zero/core/conf"
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

	ctx := svc.NewServiceContext(c)
	handler.RegisterHandlers(server, ctx)
	registerUnroutedStubs(server)

	fmt.Printf("Starting server at %s:%d...\n", c.Host, c.Port)
	server.Start()
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

// registerUnroutedStubs wires the two contract routes that don't fit the
// goctl .api JSON DSL (docs/TZ.md §7.3, FR-T3): WebSocket upgrades and a
// binary PDF download. Both currently return 501 — implementing them is
// next-phase business logic (the realtime hub and QR/PDF rendering),
// not part of this skeleton.
func registerUnroutedStubs(server *rest.Server) {
	// Plain 501, deliberately outside the errs envelope: these routes are
	// not wired to any logic yet, which is a scaffold state, not one of
	// the closed set of domain error codes in docs/TZ.md §8.1.
	notImplemented := func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "not implemented", http.StatusNotImplemented)
	}

	server.AddRoute(rest.Route{Method: http.MethodGet, Path: "/ws/guest", Handler: notImplemented})
	server.AddRoute(rest.Route{Method: http.MethodGet, Path: "/ws/staff", Handler: notImplemented})
	server.AddRoute(rest.Route{Method: http.MethodGet, Path: "/api/v1/admin/tables/qr.pdf", Handler: notImplemented})
}
