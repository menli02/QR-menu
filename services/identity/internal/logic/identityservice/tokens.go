package identityservicelogic

import (
	"context"
	"time"

	"github.com/menli02/QR-menu/proto/identity/v1"
	"github.com/menli02/QR-menu/services/identity/internal/authkey"
	"github.com/menli02/QR-menu/services/identity/internal/model"
	"github.com/menli02/QR-menu/services/identity/internal/svc"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// issueStaffTokenPair mints a fresh access+refresh pair for staff and
// persists the refresh token's hash. Shared by LoginLogic (fresh login)
// and RefreshLogic (rotation) — the returned *model.RefreshToken lets
// RefreshLogic chain the old token's replaced_by to this one's id.
func issueStaffTokenPair(ctx context.Context, svcCtx *svc.ServiceContext, staff *model.Staff) (*v1_identitypb.TokenPair, *model.RefreshToken, error) {
	accessToken, accessExpiresAt, err := authkey.MintStaff(
		svcCtx.PrivateKey, svcCtx.KID, staff.ID, staff.VenueID, staff.Role,
		time.Duration(svcCtx.Config.JWT.AccessExpireSeconds)*time.Second)
	if err != nil {
		return nil, nil, status.Errorf(codes.Internal, "mint access token: %v", err)
	}

	refreshToken, err := authkey.GenerateOpaqueToken()
	if err != nil {
		return nil, nil, status.Errorf(codes.Internal, "generate refresh token: %v", err)
	}
	refreshExpiresAt := time.Now().Add(time.Duration(svcCtx.Config.JWT.RefreshExpireSeconds) * time.Second)

	rt, err := svcCtx.RefreshTokenModel.Insert(ctx, staff.ID, authkey.HashToken(refreshToken), refreshExpiresAt)
	if err != nil {
		return nil, nil, status.Errorf(codes.Internal, "persist refresh token: %v", err)
	}

	return &v1_identitypb.TokenPair{
		AccessToken:           accessToken,
		RefreshToken:          refreshToken,
		TokenType:             "Bearer",
		AccessTokenExpiresAt:  timestamppb.New(accessExpiresAt),
		RefreshTokenExpiresAt: timestamppb.New(refreshExpiresAt),
	}, rt, nil
}
