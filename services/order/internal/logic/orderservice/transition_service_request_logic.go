package orderservicelogic

import (
	"context"

	"github.com/menli02/QR-menu/proto/order/v1"
	"github.com/menli02/QR-menu/services/order/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type TransitionServiceRequestLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewTransitionServiceRequestLogic(ctx context.Context, svcCtx *svc.ServiceContext) *TransitionServiceRequestLogic {
	return &TransitionServiceRequestLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *TransitionServiceRequestLogic) TransitionServiceRequest(in *v1_orderpb.TransitionServiceRequestRequest) (*v1_orderpb.ServiceRequest, error) {
	// todo: add your logic here and delete this line

	return &v1_orderpb.ServiceRequest{}, nil
}
