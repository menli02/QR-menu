package identityservicelogic

import (
	"context"
	"time"

	"github.com/menli02/QR-menu/proto/identity/v1"
	"github.com/menli02/QR-menu/services/identity/internal/authkey"
	"github.com/menli02/QR-menu/services/identity/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type IssueGuestTokenLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewIssueGuestTokenLogic(ctx context.Context, svcCtx *svc.ServiceContext) *IssueGuestTokenLogic {
	return &IssueGuestTokenLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// IssueGuestToken mints an anonymous guest JWT bound to
// (venue_id, table_id, guest_session_id). The caller (gateway) is trusted
// to have already validated the QR signature via catalog.ResolveTable
// (docs/TZ.md FR-O1) — this only guards against obviously-malformed calls,
// not QR/table validity, which isn't identity's concern.
func (l *IssueGuestTokenLogic) IssueGuestToken(in *v1_identitypb.IssueGuestTokenRequest) (*v1_identitypb.TokenPair, error) {
	if in.GetVenueId() == "" || in.GetTableId() == "" || in.GetGuestSessionId() == "" {
		return nil, status.Error(codes.InvalidArgument, "venue_id, table_id and guest_session_id are required")
	}

	ttl := time.Duration(in.GetTtlSeconds()) * time.Second
	if in.GetTtlSeconds() <= 0 {
		ttl = time.Duration(l.svcCtx.Config.JWT.GuestTokenTTLSeconds) * time.Second
	}

	token, expiresAt, err := authkey.MintGuest(
		l.svcCtx.PrivateKey, l.svcCtx.KID, in.GetVenueId(), in.GetTableId(), in.GetGuestSessionId(), ttl)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "mint guest token: %v", err)
	}

	return &v1_identitypb.TokenPair{
		AccessToken: token,
		// RefreshToken intentionally empty: no refresh flow for guest
		// tokens in R1 (identity.proto TokenPair comment) — a lapsed
		// token is replaced by minting a new one from the QR link again.
		TokenType:            "Bearer",
		AccessTokenExpiresAt: timestamppb.New(expiresAt),
	}, nil
}
