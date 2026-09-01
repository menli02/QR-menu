package orderservicelogic

import (
	"context"

	"github.com/menli02/QR-menu/proto/order/v1"
	"github.com/menli02/QR-menu/services/order/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListActiveTableSessionsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListActiveTableSessionsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListActiveTableSessionsLogic {
	return &ListActiveTableSessionsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ListActiveTableSessionsLogic) ListActiveTableSessions(in *v1_orderpb.ListActiveTableSessionsRequest) (*v1_orderpb.ListActiveTableSessionsResponse, error) {
	// todo: add your logic here and delete this line

	return &v1_orderpb.ListActiveTableSessionsResponse{}, nil
}
