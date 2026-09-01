package config

import "github.com/zeromicro/go-zero/zrpc"

type Config struct {
	zrpc.RpcServerConf

	// Postgres is the identity_db connection: staff, roles, refresh
	// tokens, JWT signing keys (docs/TZ.md §6, §7.1, §11).
	Postgres struct {
		DataSource string
	}

	// JWT holds the signing key material for both staff and guest tokens.
	// AccessSecret here is a non-secret local-dev placeholder only — real
	// deployments source it from a Kubernetes Secret and must rotate it
	// via a key_version'd JWKS, never a bare shared secret in a committed
	// file (docs/TZ.md §11.3, FR-T4-style rotation for token keys).
	JWT struct {
		AccessSecret         string
		AccessExpireSeconds  int64
		RefreshExpireSeconds int64
		GuestTokenTTLSeconds int64
	}
}
