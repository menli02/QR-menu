package main

import (
	"flag"
	"fmt"
	"time"

	"github.com/menli02/QR-menu/pkg/outbox"
	"github.com/menli02/QR-menu/proto/order/v1"
	"github.com/menli02/QR-menu/services/order/internal/config"
	orderserviceServer "github.com/menli02/QR-menu/services/order/internal/server/orderservice"
	"github.com/menli02/QR-menu/services/order/internal/svc"

	"github.com/zeromicro/go-zero/core/conf"
	"github.com/zeromicro/go-zero/core/service"
	"github.com/zeromicro/go-zero/zrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

var configFile = flag.String("f", "etc/order.v1.yaml", "the config file")

func main() {
	flag.Parse()

	var c config.Config
	conf.MustLoad(*configFile, &c)
	ctx := svc.NewServiceContext(c)

	s := zrpc.MustNewServer(c.RpcServerConf, func(grpcServer *grpc.Server) {
		v1_orderpb.RegisterOrderServiceServer(grpcServer, orderserviceServer.NewOrderServiceServer(ctx))

		if c.Mode == service.DevMode || c.Mode == service.TestMode {
			reflection.Register(grpcServer)
		}
	})

	// The outbox relay runs in-process alongside the RPC server rather
	// than as its own deployment: it shares this service's database
	// credentials and its rows, and one fewer moving part is worth more
	// than independent scaling for a workload this small. Several
	// replicas are still safe — the claim query uses FOR UPDATE SKIP
	// LOCKED (docs/TZ.md §8.3).
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
