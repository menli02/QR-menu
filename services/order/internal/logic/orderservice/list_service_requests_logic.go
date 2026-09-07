package orderservicelogic

import (
	"context"

	"github.com/menli02/QR-menu/proto/order/v1"
	"github.com/menli02/QR-menu/services/order/internal/apierr"
	"github.com/menli02/QR-menu/services/order/internal/model"
	"github.com/menli02/QR-menu/services/order/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListServiceRequestsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListServiceRequestsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListServiceRequestsLogic {
	return &ListServiceRequestsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// ListServiceRequests is the floor view (FR-S3): open guest requests with
// table, type and age, oldest first.
//
// It sweeps expiries first (FR-S4). Doing it on the read path means the
// floor view is always self-correcting — a request that timed out while
// nobody was looking is `expired` by the time anyone looks, with no cron
// job to deploy, monitor and forget. The sweep is a single indexed UPDATE
// over one venue's open requests, idempotent and safe to run concurrently
// with itself.
//
// The cost is honest and bounded: a venue nobody is watching accumulates
// stale-looking rows until someone opens the floor view. Nothing reads
// `expired` for money, so that is a display lag, not a correctness bug.
// A background sweeper is the obvious upgrade if the WS push for
// expiries ever needs to fire without a reader present.
func (l *ListServiceRequestsLogic) ListServiceRequests(in *v1_orderpb.ListServiceRequestsRequest) (*v1_orderpb.ListServiceRequestsResponse, error) {
	if in.GetVenueId() == "" {
		return nil, apierr.Validation("venue_id is required")
	}

	statuses := make([]string, 0, len(in.GetStatusFilter()))
	for _, s := range in.GetStatusFilter() {
		mapped, ok := requestStatusFromProto[s]
		if !ok {
			return nil, apierr.Validation("status_filter contains an unknown service request status")
		}
		statuses = append(statuses, mapped)
	}

	requests := model.NewServiceRequestModel(l.svcCtx.DB)
	if n, err := requests.ExpireOverdue(l.ctx, in.GetVenueId()); err != nil {
		// Non-fatal: the caller asked for a list, and returning a slightly
		// stale one beats returning nothing.
		l.Errorf("expire overdue service requests for venue %s: %v", in.GetVenueId(), err)
	} else if n > 0 {
		l.Infof("expired %d overdue service requests for venue %s", n, in.GetVenueId())
	}

	rows, err := requests.List(l.ctx, in.GetVenueId(), statuses)
	if err != nil {
		l.Errorf("list service requests for venue %s: %v", in.GetVenueId(), err)
		return nil, apierr.Internal("list service requests")
	}

	out := &v1_orderpb.ListServiceRequestsResponse{
		Requests: make([]*v1_orderpb.ServiceRequest, 0, len(rows)),
	}
	for i := range rows {
		out.Requests = append(out.Requests, serviceRequestToProto(&rows[i]))
	}
	return out, nil
}
