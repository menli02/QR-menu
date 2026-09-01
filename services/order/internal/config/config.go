package config

import "github.com/zeromicro/go-zero/zrpc"

type Config struct {
	zrpc.RpcServerConf

	// Postgres is the order_db connection: table/guest sessions, orders,
	// order items, service requests, bill, idempotency_key, outbox
	// (docs/TZ.md §6, §7.4). Local dev default matches
	// deploy/docker-compose.yml; production values are injected from a
	// Kubernetes Secret, never committed here.
	Postgres struct {
		DataSource string
	}

	// CatalogRpc is the etcd-discovered client used to price and validate
	// items on CreateOrder (docs/TZ.md §7.2: order -> catalog.ResolveOrderItems).
	CatalogRpc zrpc.RpcClientConf

	// KafkaBrokers feeds the transactional outbox relay that publishes
	// qrmenu.order.v1 and qrmenu.service_request.v1 events
	// (docs/TZ.md §7.2, §8.3). Field shape only for now — no producer is
	// constructed until the relay is implemented.
	KafkaBrokers []string
}
