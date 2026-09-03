package identityservicelogic

import (
	"context"
	"errors"

	"github.com/menli02/QR-menu/proto/identity/v1"
	"github.com/menli02/QR-menu/services/identity/internal/authkey"
	"github.com/menli02/QR-menu/services/identity/internal/model"
	"github.com/menli02/QR-menu/services/identity/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type RevokeLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewRevokeLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RevokeLogic {
	return &RevokeLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// Revoke is logout: it invalidates one refresh token. Idempotent and
// lenient by design — revoking a token that doesn't exist or is already
// revoked is not an error to the caller, and doesn't leak whether it ever
// existed.
func (l *RevokeLogic) Revoke(in *v1_identitypb.RevokeRequest) (*v1_identitypb.RevokeResponse, error) {
	rt, err := l.svcCtx.RefreshTokenModel.FindByHash(l.ctx, authkey.HashToken(in.GetRefreshToken()))
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			return &v1_identitypb.RevokeResponse{Revoked: true}, nil
		}
		return nil, status.Errorf(codes.Internal, "look up refresh token: %v", err)
	}
	if err := l.svcCtx.RefreshTokenModel.MarkRevoked(l.ctx, rt.ID); err != nil {
		return nil, status.Errorf(codes.Internal, "revoke refresh token: %v", err)
	}
	return &v1_identitypb.RevokeResponse{Revoked: true}, nil
}
