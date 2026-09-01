// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package admin

import (
	"context"

	"github.com/menli02/QR-menu/services/gateway/internal/svc"
	"github.com/menli02/QR-menu/services/gateway/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetDayReportLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetDayReportLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetDayReportLogic {
	return &GetDayReportLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetDayReportLogic) GetDayReport(req *types.GetDayReportReq) (resp *types.DayReport, err error) {
	// todo: add your logic here and delete this line

	return
}
