package identityservicelogic

import (
	"context"

	"github.com/menli02/QR-menu/proto/identity/v1"
	"github.com/menli02/QR-menu/services/identity/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
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
func (l *IssueGuestTokenLogic) IssueGuestToken(in *v1_identitypb.IssueGuestTokenRequest) (*v1_identitypb.TokenPair, error) {
	// todo: add your logic here and delete this line

	return &v1_identitypb.TokenPair{}, nil
}
