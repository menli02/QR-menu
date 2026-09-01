package orderservicelogic

import (
	"context"

	"github.com/menli02/QR-menu/proto/order/v1"
	"github.com/menli02/QR-menu/services/order/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type CreateServiceRequestLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCreateServiceRequestLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateServiceRequestLogic {
	return &CreateServiceRequestLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *CreateServiceRequestLogic) CreateServiceRequest(in *v1_orderpb.CreateServiceRequestRequest) (*v1_orderpb.ServiceRequest, error) {
	// todo: add your logic here and delete this line

	return &v1_orderpb.ServiceRequest{}, nil
}
