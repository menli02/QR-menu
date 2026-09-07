// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package admin

import (
	"context"

	v1_orderpb "github.com/menli02/QR-menu/proto/order/v1"
	"github.com/menli02/QR-menu/services/gateway/internal/authz"
	"github.com/menli02/QR-menu/services/gateway/internal/convert"
	"github.com/menli02/QR-menu/services/gateway/internal/rpcerr"
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

// GetDayReport is the manager's daily summary (FR-A3).
//
// An omitted date means today in the venue's own timezone, which the order
// service resolves — it holds the business-day logic, and having the
// gateway guess "today" from its own clock would disagree with how the
// orders were bucketed in the first place.
func (l *GetDayReportLogic) GetDayReport(req *types.GetDayReportReq) (resp *types.DayReport, err error) {
	claims, err := authz.Admin(l.ctx)
	if err != nil {
		return nil, err
	}

	report, err := l.svcCtx.OrderRpc.GetDayReport(l.ctx, &v1_orderpb.GetDayReportRequest{
		VenueId:      claims.VenueID,
		BusinessDate: req.Date,
	})
	if err != nil {
		return nil, rpcerr.From(err)
	}

	out := convert.DayReport(report)
	return &out, nil
}
