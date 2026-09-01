package orderservicelogic

import (
	"context"

	"github.com/menli02/QR-menu/proto/order/v1"
	"github.com/menli02/QR-menu/services/order/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListServiceRequestsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListServiceRequestsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListServiceRequestsLogic {
	return &ListServiceRequestsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ListServiceRequestsLogic) ListServiceRequests(in *v1_orderpb.ListServiceRequestsRequest) (*v1_orderpb.ListServiceRequestsResponse, error) {
	// todo: add your logic here and delete this line

	return &v1_orderpb.ListServiceRequestsResponse{}, nil
}
