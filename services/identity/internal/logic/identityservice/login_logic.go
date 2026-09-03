package identityservicelogic

import (
	"context"
	"errors"

	"github.com/menli02/QR-menu/proto/identity/v1"
	"github.com/menli02/QR-menu/services/identity/internal/model"
	"github.com/menli02/QR-menu/services/identity/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type LoginLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewLoginLogic(ctx context.Context, svcCtx *svc.ServiceContext) *LoginLogic {
	return &LoginLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// Login verifies venue-scoped staff credentials and issues a token pair:
// a signed RS256 access token (stateless, verified offline by gateway via
// ListJWKS) and an opaque, DB-backed refresh token (docs/TZ.md §7.1, §8.2).
func (l *LoginLogic) Login(in *v1_identitypb.LoginRequest) (*v1_identitypb.TokenPair, error) {
	staff, err := l.svcCtx.StaffModel.FindByVenueAndEmail(l.ctx, in.GetVenueId(), in.GetEmail())
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			// Same response whether the account doesn't exist or the
			// password is wrong: don't let a caller distinguish account
			// existence by response shape.
			return nil, status.Error(codes.Unauthenticated, "invalid credentials")
		}
		return nil, status.Errorf(codes.Internal, "look up staff: %v", err)
	}
	if !staff.IsActive || !verifyPassword(staff.PasswordHash, in.GetPassword()) {
		return nil, status.Error(codes.Unauthenticated, "invalid credentials")
	}

	pair, _, err := issueStaffTokenPair(l.ctx, l.svcCtx, staff)
	return pair, err
}
