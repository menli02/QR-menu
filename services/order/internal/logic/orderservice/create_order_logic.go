package orderservicelogic

import (
	"context"

	"github.com/menli02/QR-menu/proto/order/v1"
	"github.com/menli02/QR-menu/services/order/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type CreateOrderLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCreateOrderLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateOrderLogic {
	return &CreateOrderLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// CreateOrder is idempotent on idempotency_key (scoped to venue_id).
func (l *CreateOrderLogic) CreateOrder(in *v1_orderpb.CreateOrderRequest) (*v1_orderpb.Order, error) {
	// todo: add your logic here and delete this line

	return &v1_orderpb.Order{}, nil
}
