package config

import (
	"github.com/zeromicro/go-zero/core/stores/redis"
	"github.com/zeromicro/go-zero/rest"
	"github.com/zeromicro/go-zero/zrpc"
)

type Config struct {
	rest.RestConf

	// Downstream rpc clients, etcd-discovered (docs/TZ.md §7.1).
	CatalogRpc  zrpc.RpcClientConf
	OrderRpc    zrpc.RpcClientConf
	IdentityRpc zrpc.RpcClientConf

	// Redis backs response cache, rate limiting and the realtime pub/sub
	// fallback (docs/TZ.md §7.1, §7.3).
	Redis redis.RedisConf

	// JWKSRefreshSeconds controls how often GuestAuth/StaffAuth middleware
	// re-fetch identity.ListJWKS to verify tokens offline (docs/TZ.md §8.2).
	JWKSRefreshSeconds int64

	// KafkaBrokers feeds the realtime consumer that pushes domain events
	// to WebSocket clients (docs/TZ.md §7.3). Empty disables realtime
	// push: REST still works and sockets still connect, they just receive
	// nothing — which is a usable local-dev state, and exactly what the
	// clients' polling fallback exists for.
	KafkaBrokers []string `json:",optional"`

	// KafkaGroupPrefix and KafkaInstanceID form this pod's consumer group.
	// §7.3 gives every gateway instance its *own* group so all of them see
	// every event; InstanceID defaults to HOSTNAME (the pod name under
	// Kubernetes), which is almost always what you want — so it is
	// optional here rather than something every deployment has to set.
	KafkaGroupPrefix string `json:",optional"`
	KafkaInstanceID  string `json:",optional"`

	// WSAllowedOrigins is the WebSocket handshake origin allow-list.
	// Browsers do not apply the same-origin policy to WebSocket upgrades,
	// so this is a real control, not a formality. Empty permits
	// same-origin handshakes only.
	WSAllowedOrigins []string `json:",optional"`
}
