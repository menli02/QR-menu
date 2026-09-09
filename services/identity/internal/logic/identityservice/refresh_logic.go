package identityservicelogic

import (
	"context"
	"errors"
	"time"

	"github.com/menli02/QR-menu/proto/identity/v1"
	"github.com/menli02/QR-menu/services/identity/internal/authkey"
	"github.com/menli02/QR-menu/services/identity/internal/model"
	"github.com/menli02/QR-menu/services/identity/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type RefreshLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewRefreshLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RefreshLogic {
	return &RefreshLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// Refresh rotates a refresh token: the presented token is revoked and
// chained (replaced_by) to a newly issued one. Seeing an
// already-revoked token presented again is treated as reuse — a strong
// signal of a stolen/duplicated token — and revokes every other active
// refresh token for that staff member defensively.
func (l *RefreshLogic) Refresh(in *v1_identitypb.RefreshRequest) (*v1_identitypb.TokenPair, error) {
	hash := authkey.HashToken(in.GetRefreshToken())
	rt, err := l.svcCtx.RefreshTokenModel.FindByHash(l.ctx, hash)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			return nil, status.Error(codes.Unauthenticated, "invalid refresh token")
		}
		return nil, status.Errorf(codes.Internal, "look up refresh token: %v", err)
	}

	if rt.RevokedAt.Valid {
		if err := l.svcCtx.RefreshTokenModel.RevokeAllForStaff(l.ctx, rt.StaffID); err != nil {
			l.Errorf("revoke all refresh tokens for staff %s after reuse: %v", rt.StaffID, err)
		}
		return nil, status.Error(codes.Unauthenticated, "invalid refresh token")
	}
	if time.Now().After(rt.ExpiresAt) {
		return nil, status.Error(codes.Unauthenticated, "invalid refresh token")
	}

	// Unscoped by design: the authenticated refresh-token row is what
	// identifies this staff member, so there is no venue to scope to.
	staff, err := l.svcCtx.StaffModel.FindByIDUnscoped(l.ctx, rt.StaffID)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			return nil, status.Error(codes.Unauthenticated, "invalid refresh token")
		}
		return nil, status.Errorf(codes.Internal, "look up staff: %v", err)
	}
	if !staff.IsActive {
		return nil, status.Error(codes.Unauthenticated, "invalid refresh token")
	}

	pair, newRT, err := issueStaffTokenPair(l.ctx, l.svcCtx, staff)
	if err != nil {
		return nil, err
	}
	if err := l.svcCtx.RefreshTokenModel.MarkReplaced(l.ctx, rt.ID, newRT.ID); err != nil {
		return nil, status.Errorf(codes.Internal, "chain refresh token rotation: %v", err)
	}
	return pair, nil
}
