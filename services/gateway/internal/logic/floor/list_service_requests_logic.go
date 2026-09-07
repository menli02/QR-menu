// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package floor

import (
	"context"

	v1_orderpb "github.com/menli02/QR-menu/proto/order/v1"
	"github.com/menli02/QR-menu/services/gateway/internal/authz"
	"github.com/menli02/QR-menu/services/gateway/internal/convert"
	"github.com/menli02/QR-menu/services/gateway/internal/errs"
	"github.com/menli02/QR-menu/services/gateway/internal/rpcerr"
	"github.com/menli02/QR-menu/services/gateway/internal/svc"
	"github.com/menli02/QR-menu/services/gateway/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListServiceRequestsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewListServiceRequestsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListServiceRequestsLogic {
	return &ListServiceRequestsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// ListServiceRequests is the floor view (FR-S3): who is waiting, at which
// table, for how long.
//
// The order service sweeps FR-S4 expiries on this read, so the list is
// self-correcting — a request that timed out while nobody was watching is
// already `expired` by the time anyone looks.
func (l *ListServiceRequestsLogic) ListServiceRequests(req *types.ListServiceRequestsReq) (resp *types.ListServiceRequestsResp, err error) {
	claims, err := authz.Staff(l.ctx)
	if err != nil {
		return nil, err
	}

	var filter []v1_orderpb.ServiceRequestStatus
	switch req.Status {
	case "", "open":
		// Empty filter: the order service defaults to open.
	case "all":
		for _, s := range convert.ServiceRequestStatuses {
			filter = append(filter, s)
		}
	default:
		return nil, errs.New(errs.CodeValidationFailed, "status must be open or all")
	}

	list, err := l.svcCtx.OrderRpc.ListServiceRequests(l.ctx, &v1_orderpb.ListServiceRequestsRequest{
		VenueId:      claims.VenueID,
		StatusFilter: filter,
	})
	if err != nil {
		return nil, rpcerr.From(err)
	}

	return &types.ListServiceRequestsResp{Requests: convert.ServiceRequests(list.GetRequests())}, nil
}
