package svc

import (
	"crypto/rsa"

	"github.com/menli02/QR-menu/services/identity/internal/authkey"
	"github.com/menli02/QR-menu/services/identity/internal/config"
	"github.com/menli02/QR-menu/services/identity/internal/model"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/postgres"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

type ServiceContext struct {
	Config config.Config

	// DB is a lazy connection pool: dialing happens on first query, so a
	// transient Postgres outage at boot never crash-loops this service —
	// readiness should be reported by a DB ping, not by startup success.
	DB sqlx.SqlConn

	StaffModel        *model.StaffModel
	RefreshTokenModel *model.RefreshTokenModel
	SigningKeyModel   *model.SigningKeyModel

	// PrivateKey signs every staff and guest token this instance issues;
	// KID is its stable identifier, embedded in each token's header so a
	// verifier can pick the matching JWK out of ListJWKS. Loaded (or
	// generated, in local dev) synchronously at startup: unlike DB, this
	// is pure local file I/O, so there's no lazy-init benefit to chasing —
	// see authkey.LoadOrCreatePrivateKey. There's also nothing useful this
	// service can do without it, so a failure here is fatal at boot,
	// unlike a transient DB outage.
	PrivateKey *rsa.PrivateKey
	KID        string
}

func NewServiceContext(c config.Config) *ServiceContext {
	db := postgres.New(c.Postgres.DataSource)

	priv, err := authkey.LoadOrCreatePrivateKey(c.JWT.PrivateKeyPath)
	logx.Must(err)

	return &ServiceContext{
		Config:            c,
		DB:                db,
		StaffModel:        model.NewStaffModel(db),
		RefreshTokenModel: model.NewRefreshTokenModel(db),
		SigningKeyModel:   model.NewSigningKeyModel(db),
		PrivateKey:        priv,
		KID:               authkey.KeyID(&priv.PublicKey),
	}
}
