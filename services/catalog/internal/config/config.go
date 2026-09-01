package config

import "github.com/zeromicro/go-zero/zrpc"

type Config struct {
	zrpc.RpcServerConf

	// Postgres is the catalog_db connection (docs/TZ.md §6, §7.1). Local
	// dev default matches deploy/docker-compose.yml; production values are
	// injected from a Kubernetes Secret, never committed here.
	Postgres struct {
		DataSource string
	}

	// MinIO holds object storage settings for menu item images (FR-C2).
	// Field shape only for now — no client is constructed until image
	// upload/serve logic is implemented.
	MinIO struct {
		Endpoint  string
		AccessKey string
		SecretKey string
		Bucket    string
		UseSSL    bool
	}

	// KafkaBrokers feeds the transactional outbox relay that publishes
	// qrmenu.catalog.v1 events (docs/TZ.md §7.2, §8.3). Field shape only
	// for now — no producer is constructed until the relay is implemented.
	KafkaBrokers []string
}
