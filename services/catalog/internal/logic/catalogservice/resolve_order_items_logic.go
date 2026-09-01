package catalogservicelogic

import (
	"context"

	"github.com/menli02/QR-menu/proto/catalog/v1"
	"github.com/menli02/QR-menu/services/catalog/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type ResolveOrderItemsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewResolveOrderItemsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ResolveOrderItemsLogic {
	return &ResolveOrderItemsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// ResolveOrderItems re-prices and validates availability of requested items
func (l *ResolveOrderItemsLogic) ResolveOrderItems(in *v1_catalogpb.ResolveOrderItemsRequest) (*v1_catalogpb.ResolveOrderItemsResponse, error) {
	// todo: add your logic here and delete this line

	return &v1_catalogpb.ResolveOrderItemsResponse{}, nil
}
