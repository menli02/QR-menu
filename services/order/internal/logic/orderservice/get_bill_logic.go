package orderservicelogic

import (
	"context"

	"github.com/menli02/QR-menu/proto/order/v1"
	"github.com/menli02/QR-menu/services/order/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetBillLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetBillLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetBillLogic {
	return &GetBillLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *GetBillLogic) GetBill(in *v1_orderpb.GetBillRequest) (*v1_orderpb.Bill, error) {
	// todo: add your logic here and delete this line

	return &v1_orderpb.Bill{}, nil
}
