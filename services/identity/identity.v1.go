package main

import (
	"flag"
	"fmt"

	"github.com/menli02/QR-menu/pkg/health"
	"github.com/menli02/QR-menu/proto/identity/v1"
	"github.com/menli02/QR-menu/services/identity/internal/config"
	identityserviceServer "github.com/menli02/QR-menu/services/identity/internal/server/identityservice"
	"github.com/menli02/QR-menu/services/identity/internal/svc"

	"github.com/zeromicro/go-zero/core/conf"
	"github.com/zeromicro/go-zero/core/service"
	"github.com/zeromicro/go-zero/zrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

var configFile = flag.String("f", "etc/identity.v1.yaml", "the config file")

func main() {
	flag.Parse()

	var c config.Config
	conf.MustLoad(*configFile, &c)
	ctx := svc.NewServiceContext(c)

	s := zrpc.MustNewServer(c.RpcServerConf, func(grpcServer *grpc.Server) {
		v1_identitypb.RegisterIdentityServiceServer(grpcServer, identityserviceServer.NewIdentityServiceServer(ctx))

		if c.Mode == service.DevMode || c.Mode == service.TestMode {
			reflection.Register(grpcServer)
		}
	})

	// See services/order/order.v1.go for why this drives the startup probe
	// rather than the readiness probe.
	ready := health.NewServer(c.ReadinessPort)
	ready.Register("postgres", health.PostgresCheck(ctx.DB))

	group := service.NewServiceGroup()
	defer group.Stop()
	group.Add(s)
	group.Add(ready)

	fmt.Printf("Starting rpc server at %s...\n", c.ListenOn)
	group.Start()
}
