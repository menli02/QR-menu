// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package floor

import (
	"context"

	"github.com/menli02/QR-menu/services/gateway/internal/svc"
	"github.com/menli02/QR-menu/services/gateway/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListServiceRequestsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewListServiceRequestsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListServiceRequestsLogic {
	return &ListServiceRequestsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ListServiceRequestsLogic) ListServiceRequests(req *types.ListServiceRequestsReq) (resp *types.ListServiceRequestsResp, err error) {
	// todo: add your logic here and delete this line

	return
}
