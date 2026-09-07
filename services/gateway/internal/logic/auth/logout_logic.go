// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package auth

import (
	"context"

	v1_identitypb "github.com/menli02/QR-menu/proto/identity/v1"
	"github.com/menli02/QR-menu/services/gateway/internal/errs"
	"github.com/menli02/QR-menu/services/gateway/internal/rpcerr"
	"github.com/menli02/QR-menu/services/gateway/internal/svc"
	"github.com/menli02/QR-menu/services/gateway/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type LogoutLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewLogoutLogic(ctx context.Context, svcCtx *svc.ServiceContext) *LogoutLogic {
	return &LogoutLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// Logout revokes a refresh token.
//
// Revoking a token that was already invalid reports revoked=true rather
// than an error. Logout is idempotent by nature: the caller's intent is
// "this token must not work any more", and if it already doesn't, that
// intent is satisfied. Returning 404 would also confirm to an attacker
// which tokens are live.
//
// The access token is untouched — it is a stateless JWT and stays valid
// until it expires (15 min). That is the accepted trade of offline
// verification (docs/TZ.md §7.3: the gateway verifies tokens against JWKS
// without calling identity), and it is why access TTLs are short.
func (l *LogoutLogic) Logout(req *types.LogoutReq) (resp *types.LogoutResp, err error) {
	if req.RefreshToken == "" {
		return nil, errs.New(errs.CodeValidationFailed, "refreshToken is required")
	}

	revoked, err := l.svcCtx.IdentityRpc.Revoke(l.ctx, &v1_identitypb.RevokeRequest{
		RefreshToken: req.RefreshToken,
	})
	if err != nil {
		converted := rpcerr.From(err)
		if converted.Code == errs.CodeNotFound {
			return &types.LogoutResp{Revoked: true}, nil
		}
		return nil, converted
	}

	return &types.LogoutResp{Revoked: revoked.GetRevoked()}, nil
}
