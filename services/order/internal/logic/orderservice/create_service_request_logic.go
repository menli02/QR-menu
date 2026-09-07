package orderservicelogic

import (
	"context"
	"unicode/utf8"

	"github.com/menli02/QR-menu/proto/order/v1"
	"github.com/menli02/QR-menu/services/order/internal/apierr"
	"github.com/menli02/QR-menu/services/order/internal/model"
	"github.com/menli02/QR-menu/services/order/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

// noteMaxLen bounds FR-S7's payment-method hint. It is not a venue
// setting — no field carries one — so it is a fixed ceiling that exists to
// stop an unbounded string reaching the floor view.
const noteMaxLen = 200

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

// CreateServiceRequest is FR-S1 (call waiter / request bill) with FR-S2's
// rate limit and FR-S7's payment hint.
//
// Two layers of duplicate suppression, deliberately, because they answer
// different questions:
//
//   - The idempotency key handles "the same request arrived twice"
//     (a retry over a flaky connection).
//   - CreateOrGetOpen handles "the guest pressed the button again"
//     (a new request, same intent). That one returns the existing open
//     request rather than an error — FR-S2 says a repeat returns the
//     existing one, and a guest tapping twice has done nothing wrong.
func (l *CreateServiceRequestLogic) CreateServiceRequest(in *v1_orderpb.CreateServiceRequestRequest) (*v1_orderpb.ServiceRequest, error) {
	if in.GetVenueId() == "" || in.GetTableId() == "" {
		return nil, apierr.Validation("venue_id and table_id are required")
	}
	reqType, ok := requestTypeFromProto[in.GetType()]
	if !ok {
		return nil, apierr.Validation("type must be call_waiter or request_bill")
	}
	if n := utf8.RuneCountInString(in.GetNote()); n > noteMaxLen {
		return nil, apierr.Validation("note is too long")
	}

	settings, err := l.svcCtx.Venue.Get(l.ctx, in.GetVenueId())
	if err != nil {
		return nil, catalogError(err, "load venue settings")
	}

	fingerprint := model.Fingerprint(in.GetVenueId(), in.GetTableId(), reqType, in.GetNote())

	empty := func() *v1_orderpb.ServiceRequest { return &v1_orderpb.ServiceRequest{} }
	if replay, found, err := peekIdempotent(l.ctx, l.svcCtx.DB, endpointCreateServiceRequest,
		in.GetVenueId(), in.GetIdempotencyKey(), fingerprint, empty); err != nil {
		return nil, err
	} else if found {
		return replay, nil
	}

	return runIdempotent(l.ctx, l.svcCtx.DB, endpointCreateServiceRequest,
		in.GetVenueId(), in.GetIdempotencyKey(), fingerprint, empty,
		func(ctx context.Context, s sqlx.Session) (*v1_orderpb.ServiceRequest, error) {
			return l.create(ctx, s, in, reqType, settings.Currency)
		})
}

func (l *CreateServiceRequestLogic) create(
	ctx context.Context,
	s sqlx.Session,
	in *v1_orderpb.CreateServiceRequestRequest,
	reqType, currency string,
) (*v1_orderpb.ServiceRequest, error) {
	// A guest can call a waiter before ordering anything, so this goes
	// through the same open-or-join path CreateOrder uses rather than
	// requiring a session to already exist (see the migration's comment
	// on table_sessions).
	session, err := model.NewTableSessionModel(s).OpenOrJoin(ctx, in.GetVenueId(), in.GetTableId(), currency)
	if err != nil {
		l.Errorf("open or join table session for service request: %v", err)
		return nil, apierr.Internal("open table session")
	}

	// A client that names a session must name the current one. A stale id
	// means the guest's page predates a table turnover, and silently
	// retargeting their request at the new party's session would be worse
	// than telling them to reload.
	if want := in.GetTableSessionId(); want != "" && want != session.ID {
		return nil, apierr.SessionClosed("this table session has ended; reload to continue")
	}

	request, created, err := model.NewServiceRequestModel(s).CreateOrGetOpen(
		ctx, in.GetVenueId(), in.GetTableId(), session.ID, reqType, in.GetNote(), l.svcCtx.ServiceRequestTTL)
	if err != nil {
		l.Errorf("create service request: %v", err)
		return nil, apierr.Internal("create service request")
	}

	// No event for a coalesced repeat: nothing changed, and re-announcing
	// it would make the floor view chirp every time a guest taps.
	if created {
		if err := model.NewOutboxModel(s).Insert(ctx, eventServiceRequestCreated, in.GetVenueId(),
			model.TopicServiceRequest, request.TableID,
			serviceRequestCreatedPayload{
				ServiceRequestID: request.ID,
				TableID:          request.TableID,
				TableSessionID:   request.TableSessionID,
				Type:             request.Type,
				Note:             request.Note.String,
			}, traceID(ctx)); err != nil {
			l.Errorf("write service_request.created outbox row: %v", err)
			return nil, apierr.Internal("write outbox row")
		}
	}

	return serviceRequestToProto(request), nil
}
