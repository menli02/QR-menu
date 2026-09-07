package main

import (
	"flag"
	"fmt"
	"time"

	"github.com/menli02/QR-menu/pkg/outbox"
	"github.com/menli02/QR-menu/proto/catalog/v1"
	"github.com/menli02/QR-menu/services/catalog/internal/config"
	catalogserviceServer "github.com/menli02/QR-menu/services/catalog/internal/server/catalogservice"
	"github.com/menli02/QR-menu/services/catalog/internal/svc"

	"github.com/zeromicro/go-zero/core/conf"
	"github.com/zeromicro/go-zero/core/service"
	"github.com/zeromicro/go-zero/zrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

var configFile = flag.String("f", "etc/catalog.v1.yaml", "the config file")

func main() {
	flag.Parse()

	var c config.Config
	conf.MustLoad(*configFile, &c)
	ctx := svc.NewServiceContext(c)

	s := zrpc.MustNewServer(c.RpcServerConf, func(grpcServer *grpc.Server) {
		v1_catalogpb.RegisterCatalogServiceServer(grpcServer, catalogserviceServer.NewCatalogServiceServer(ctx))

		if c.Mode == service.DevMode || c.Mode == service.TestMode {
			reflection.Register(grpcServer)
		}
	})

	// Drains catalog's outbox into qrmenu.catalog.v1 — the path behind
	// FR-C4's "a stop-list change reaches guest clients within 5s". See
	// pkg/outbox and services/order/order.v1.go for why the relay is
	// in-process.
	relay := outbox.New(ctx.DB, outbox.Config{
		Brokers:      c.KafkaBrokers,
		PollInterval: time.Duration(c.Outbox.PollIntervalMs) * time.Millisecond,
		BatchSize:    c.Outbox.BatchSize,
		MaxAttempts:  c.Outbox.MaxAttempts,
		Source:       c.Name,
	})

	group := service.NewServiceGroup()
	defer group.Stop()
	group.Add(s)
	group.Add(relay)

	fmt.Printf("Starting rpc server at %s...\n", c.ListenOn)
	group.Start()
}
