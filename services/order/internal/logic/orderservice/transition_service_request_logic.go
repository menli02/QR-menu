package orderservicelogic

import (
	"context"
	"fmt"

	"github.com/menli02/QR-menu/proto/order/v1"
	"github.com/menli02/QR-menu/services/order/internal/apierr"
	"github.com/menli02/QR-menu/services/order/internal/model"
	"github.com/menli02/QR-menu/services/order/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

// serviceRequestTransitions is FR-S3's staff workflow: acknowledge, then
// resolve; or resolve straight away for a request handled on the spot.
// `expired` is reachable only from the FR-S4 sweep, never from a staff
// action, so it is not a target here.
var serviceRequestTransitions = map[string]map[string]bool{
	model.RequestOpen: {
		model.RequestAcknowledged: true,
		model.RequestResolved:     true,
	},
	model.RequestAcknowledged: {
		model.RequestResolved: true,
	},
	model.RequestResolved: {},
	model.RequestExpired: {
		// A request that timed out and *then* got attended to should be
		// recordable as resolved rather than left looking abandoned.
		model.RequestResolved: true,
	},
}

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

// TransitionServiceRequest is a staff acknowledge/resolve (FR-S3).
//
// Like CloseTableSession, this RPC carries no idempotency_key. It doesn't
// need one: the write is a compare-and-set on the current status, and a
// repeat of a transition that already landed is reported as success
// rather than a conflict — two waiters both tapping "acknowledge" is
// normal, and neither should see an error.
func (l *TransitionServiceRequestLogic) TransitionServiceRequest(in *v1_orderpb.TransitionServiceRequestRequest) (*v1_orderpb.ServiceRequest, error) {
	if in.GetVenueId() == "" || in.GetServiceRequestId() == "" {
		return nil, apierr.Validation("venue_id and service_request_id are required")
	}
	to, ok := requestStatusFromProto[in.GetTo()]
	if !ok {
		return nil, apierr.Validation("to must be a known service request status")
	}
	if to != model.RequestAcknowledged && to != model.RequestResolved {
		return nil, apierr.Validation("staff may only acknowledge or resolve a service request")
	}

	var out *model.ServiceRequest
	err := l.svcCtx.DB.TransactCtx(l.ctx, func(ctx context.Context, s sqlx.Session) error {
		requests := model.NewServiceRequestModel(s)

		current, err := requests.FindByID(ctx, in.GetVenueId(), in.GetServiceRequestId())
		if err != nil {
			if isNotFound(err) {
				return apierr.NotFound("service request not found")
			}
			l.Errorf("look up service request %s: %v", in.GetServiceRequestId(), err)
			return apierr.Internal("look up service request")
		}

		if current.Status == to {
			out = current
			return nil
		}
		if !serviceRequestTransitions[current.Status][to] {
			return apierr.InvalidTransition(fmt.Sprintf(
				"cannot move a service request from %s to %s", current.Status, to))
		}

		out, err = requests.UpdateStatus(ctx, in.GetVenueId(), in.GetServiceRequestId(), to, current.Status)
		if err != nil {
			if isNotFound(err) {
				return apierr.InvalidTransition("the service request changed while this request was in flight")
			}
			l.Errorf("transition service request %s: %v", in.GetServiceRequestId(), err)
			return apierr.Internal("transition service request")
		}

		return model.NewOutboxModel(s).Insert(ctx, eventServiceRequestTransited, in.GetVenueId(),
			model.TopicServiceRequest, out.TableID,
			serviceRequestTransitionedPayload{
				ServiceRequestID: out.ID,
				TableID:          out.TableID,
				TableSessionID:   out.TableSessionID,
				Type:             out.Type,
				From:             current.Status,
				To:               to,
				ActorStaffID:     in.GetActorStaffId(),
			}, traceID(ctx))
	})
	if err != nil {
		return nil, err
	}
	return serviceRequestToProto(out), nil
}
