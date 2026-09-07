// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package auth

import (
	"context"

	v1_identitypb "github.com/menli02/QR-menu/proto/identity/v1"
	"github.com/menli02/QR-menu/services/gateway/internal/convert"
	"github.com/menli02/QR-menu/services/gateway/internal/errs"
	"github.com/menli02/QR-menu/services/gateway/internal/rpcerr"
	"github.com/menli02/QR-menu/services/gateway/internal/svc"
	"github.com/menli02/QR-menu/services/gateway/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type RefreshLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewRefreshLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RefreshLogic {
	return &RefreshLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// Refresh rotates a refresh token for a new pair.
//
// Every rejection collapses to UNAUTHENTICATED on purpose. Identity
// distinguishes an unknown token from a *reused* one — the latter means a
// rotated-away token was replayed, so it revokes the whole chain — but
// telling the caller which of those happened would help an attacker
// holding a stolen token work out whether the real user has since used it.
// The security response happens server-side; the client just sees "log in
// again".
func (l *RefreshLogic) Refresh(req *types.RefreshReq) (resp *types.TokenResp, err error) {
	if req.RefreshToken == "" {
		return nil, errs.New(errs.CodeValidationFailed, "refreshToken is required")
	}

	pair, err := l.svcCtx.IdentityRpc.Refresh(l.ctx, &v1_identitypb.RefreshRequest{
		RefreshToken: req.RefreshToken,
	})
	if err != nil {
		converted := rpcerr.From(err)
		if converted.Code != errs.CodeInternal {
			return nil, errs.New(errs.CodeUnauthenticated, "refresh token is not valid")
		}
		return nil, converted
	}

	out := convert.TokenPair(pair)
	return &out, nil
}
