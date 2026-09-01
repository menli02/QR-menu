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
}
