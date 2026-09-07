// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package auth

import (
	"context"
	"strings"

	v1_catalogpb "github.com/menli02/QR-menu/proto/catalog/v1"
	v1_identitypb "github.com/menli02/QR-menu/proto/identity/v1"
	"github.com/menli02/QR-menu/services/gateway/internal/convert"
	"github.com/menli02/QR-menu/services/gateway/internal/errs"
	"github.com/menli02/QR-menu/services/gateway/internal/rpcerr"
	"github.com/menli02/QR-menu/services/gateway/internal/svc"
	"github.com/menli02/QR-menu/services/gateway/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type LoginLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewLoginLogic(ctx context.Context, svcCtx *svc.ServiceContext) *LoginLogic {
	return &LoginLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// Login exchanges venue slug + credentials for a token pair (§8.1).
//
// It is two calls: catalog resolves the public slug to a venue id (that
// table is catalog's), then identity authenticates within that venue.
//
// Both failures — unknown venue and bad credentials — return the same
// UNAUTHENTICATED response. Distinguishing them would let anyone probe
// which venue slugs exist and which emails are registered, and the honest
// answer to "was it the venue, the email or the password?" is one a login
// form should never give.
func (l *LoginLogic) Login(req *types.LoginReq) (resp *types.TokenResp, err error) {
	slug := strings.TrimSpace(req.VenueSlug)
	email := strings.TrimSpace(req.Email)
	if slug == "" || email == "" || req.Password == "" {
		return nil, errs.New(errs.CodeValidationFailed, "venueSlug, email and password are required")
	}

	venue, err := l.svcCtx.CatalogRpc.ResolveVenueBySlug(l.ctx, &v1_catalogpb.ResolveVenueBySlugRequest{Slug: slug})
	if err != nil {
		converted := rpcerr.FromCatalog(err)
		if converted.Code == errs.CodeNotFound {
			l.Infof("login attempt for unknown venue slug %q", slug)
			return nil, invalidCredentials()
		}
		return nil, converted
	}

	pair, err := l.svcCtx.IdentityRpc.Login(l.ctx, &v1_identitypb.LoginRequest{
		VenueId:  venue.GetVenueId(),
		Email:    email,
		Password: req.Password,
	})
	if err != nil {
		converted := rpcerr.From(err)
		// Identity distinguishes "no such staff" from "wrong password"
		// internally for its own logs; the client gets neither.
		switch converted.Code {
		case errs.CodeNotFound, errs.CodeUnauthenticated, errs.CodeForbidden, errs.CodeValidationFailed:
			return nil, invalidCredentials()
		}
		return nil, converted
	}

	out := convert.TokenPair(pair)
	return &out, nil
}

func invalidCredentials() *errs.Error {
	return errs.New(errs.CodeUnauthenticated, "invalid venue, email or password")
}
