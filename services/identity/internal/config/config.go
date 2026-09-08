package config

import "github.com/zeromicro/go-zero/zrpc"

type Config struct {
	zrpc.RpcServerConf

	// Postgres is the identity_db connection: staff, roles, refresh
	// tokens, JWT signing-key metadata (docs/TZ.md §6, §7.1, §11).
	Postgres struct {
		DataSource string
	}

	// JWT signing is asymmetric (RS256) by design, not a shared secret:
	// ListJWKS is a public discovery endpoint so gateway instances can
	// verify tokens offline (docs/TZ.md §8.2), and the JWK message in
	// identity.proto only carries RSA/EC *public* key fields (n/e or
	// crv/x/y) — there is no field for a symmetric secret, because
	// publishing one over that same public endpoint would let anyone
	// forge tokens.
	JWT struct {
		// PrivateKeyPath is a PEM-encoded RSA private key. In production
		// this file is a mounted Kubernetes Secret volume — never a value
		// baked into this config (docs/TZ.md §7.1, §11). If the file
		// doesn't exist yet, identity generates one and writes it here on
		// first boot, so local-dev restarts reuse the same key instead of
		// invalidating every previously issued token.
		PrivateKeyPath string

		AccessExpireSeconds  int64
		RefreshExpireSeconds int64
		GuestTokenTTLSeconds int64
	}

	// ReadinessPort serves GET /readyz, the dependency-aware check behind
	// the Kubernetes startup probe (see pkg/health). Defaults to 6061 when
	// unset — 6060 is go-zero's own admin server.
	ReadinessPort int `json:",default=6061"`
}
