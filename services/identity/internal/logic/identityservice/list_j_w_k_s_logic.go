package identityservicelogic

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/menli02/QR-menu/proto/identity/v1"
	"github.com/menli02/QR-menu/services/identity/internal/authkey"
	"github.com/menli02/QR-menu/services/identity/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type ListJWKSLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListJWKSLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListJWKSLogic {
	return &ListJWKSLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// jwkPublicFields is the on-disk shape of signing_keys.public_jwk.
type jwkPublicFields struct {
	N string `json:"n"`
	E string `json:"e"`
}

// ListJWKS is the public discovery endpoint gateway instances poll to
// verify staff/guest tokens offline (docs/TZ.md §8.2), so gateway never
// needs a round trip to identity per request.
//
// It also registers this instance's own signing key on the way through
// (EnsureActive is idempotent — see that method's comment) rather than at
// service startup: identity's DB connection is intentionally lazy (see
// service_context.go), so nothing here can turn a transient DB outage
// into a crash-looping boot.
func (l *ListJWKSLogic) ListJWKS(in *v1_identitypb.ListJWKSRequest) (*v1_identitypb.JWKS, error) {
	n, e := authkey.PublicJWKFields(&l.svcCtx.PrivateKey.PublicKey)
	fields, err := json.Marshal(jwkPublicFields{N: n, E: e})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "marshal public key: %v", err)
	}
	if err := l.svcCtx.SigningKeyModel.EnsureActive(l.ctx, l.svcCtx.KID, "RSA", "RS256", fields); err != nil {
		return nil, status.Errorf(codes.Internal, "register signing key: %v", err)
	}

	rows, err := l.svcCtx.SigningKeyModel.List(l.ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list signing keys: %v", err)
	}

	keys := make([]*v1_identitypb.JWK, 0, len(rows))
	for _, row := range rows {
		var f jwkPublicFields
		if err := json.Unmarshal(row.PublicJWK, &f); err != nil {
			l.Errorf("signing key %s has unparseable public_jwk, skipping: %v", row.Kid, err)
			continue
		}
		keys = append(keys, &v1_identitypb.JWK{
			Kid: row.Kid,
			Kty: row.Kty,
			Use: "sig",
			Alg: row.Alg,
			N:   f.N,
			E:   f.E,
		})
	}
	if len(keys) == 0 {
		// Should be unreachable right after EnsureActive above; surfaced
		// loudly rather than silently returning an empty JWKS, which
		// would make every gateway instance start rejecting all tokens.
		return nil, status.Error(codes.Internal, fmt.Sprintf("no usable signing keys after registering kid %s", l.svcCtx.KID))
	}
	return &v1_identitypb.JWKS{Keys: keys}, nil
}
