package orderservicelogic

import (
	"context"

	"github.com/menli02/QR-menu/proto/order/v1"
	"github.com/menli02/QR-menu/services/order/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetDayReportLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetDayReportLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetDayReportLogic {
	return &GetDayReportLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *GetDayReportLogic) GetDayReport(in *v1_orderpb.GetDayReportRequest) (*v1_orderpb.DayReport, error) {
	// todo: add your logic here and delete this line

	return &v1_orderpb.DayReport{}, nil
}
