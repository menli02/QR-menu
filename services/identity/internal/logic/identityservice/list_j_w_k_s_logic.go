package identityservicelogic

import (
	"context"

	"github.com/menli02/QR-menu/proto/identity/v1"
	"github.com/menli02/QR-menu/services/identity/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
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

// ListJWKS exposes the public signing keys so gateway instances can
func (l *ListJWKSLogic) ListJWKS(in *v1_identitypb.ListJWKSRequest) (*v1_identitypb.JWKS, error) {
	// todo: add your logic here and delete this line

	return &v1_identitypb.JWKS{}, nil
}
