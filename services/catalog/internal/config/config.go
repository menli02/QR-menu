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
	// qrmenu.catalog.v1 events (docs/TZ.md §7.2, §8.3). Empty disables
	// the relay, which is the right local-dev default: outbox rows still
	// accumulate and can be inspected, nothing tries to reach a broker
	// that isn't there.
	KafkaBrokers []string `json:",optional"`

	// Outbox tunes the relay goroutine (see pkg/outbox). Defaults are
	// applied for any field left at zero.
	Outbox struct {
		PollIntervalMs int `json:",optional"`
		BatchSize      int `json:",optional"`
		MaxAttempts    int `json:",optional"`
	}
}
