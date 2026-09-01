package orderservicelogic

import (
	"context"

	"github.com/menli02/QR-menu/proto/order/v1"
	"github.com/menli02/QR-menu/services/order/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type TransitionOrderLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewTransitionOrderLogic(ctx context.Context, svcCtx *svc.ServiceContext) *TransitionOrderLogic {
	return &TransitionOrderLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *TransitionOrderLogic) TransitionOrder(in *v1_orderpb.TransitionOrderRequest) (*v1_orderpb.Order, error) {
	// todo: add your logic here and delete this line

	return &v1_orderpb.Order{}, nil
}
