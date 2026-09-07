package orderservicelogic

import (
	"context"

	"github.com/menli02/QR-menu/proto/order/v1"
	"github.com/menli02/QR-menu/services/order/internal/apierr"
	"github.com/menli02/QR-menu/services/order/internal/model"
	"github.com/menli02/QR-menu/services/order/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

type CloseTableSessionLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCloseTableSessionLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CloseTableSessionLogic {
	return &CloseTableSessionLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// CloseTableSession marks the bill paid and archives the session (FR-S6).
// R1 tracks the method only — not amount received or change.
//
// CloseTableSessionRequest carries no idempotency_key, unlike the four
// write RPCs that do. That is fine here and does not need one: the close
// is a conditional UPDATE on status = 'open', so a duplicate request
// finds nothing to update. What a retry gets is reported honestly —
// FAILED_PRECONDITION, not a fabricated success — because a second close
// usually means two staff are both settling the same table and the
// second one should be told.
//
// Three things happen together, in one transaction, or none do:
// the session closes, the party's guest tokens are revoked, and any
// service request still open on the table is resolved.
func (l *CloseTableSessionLogic) CloseTableSession(in *v1_orderpb.CloseTableSessionRequest) (*v1_orderpb.TableSession, error) {
	if in.GetVenueId() == "" || in.GetTableSessionId() == "" {
		return nil, apierr.Validation("venue_id and table_session_id are required")
	}
	method, ok := paymentMethodFromProto[in.GetPaymentMethod()]
	if !ok {
		return nil, apierr.Validation("payment_method must be cash, card_terminal or other")
	}

	var closed *model.TableSession
	err := l.svcCtx.DB.TransactCtx(l.ctx, func(ctx context.Context, s sqlx.Session) error {
		sessions := model.NewTableSessionModel(s)
		outbox := model.NewOutboxModel(s)

		var err error
		closed, err = sessions.Close(ctx, in.GetVenueId(), in.GetTableSessionId(), method, in.GetActorStaffId())
		if err != nil {
			if isNotFound(err) {
				return l.explainCloseFailure(ctx, s, in)
			}
			l.Errorf("close table session %s: %v", in.GetTableSessionId(), err)
			return apierr.Internal("close table session")
		}

		if err := model.NewGuestSessionModel(s).RevokeForTableSession(ctx, closed.ID); err != nil {
			l.Errorf("revoke guest sessions for %s: %v", closed.ID, err)
			return apierr.Internal("revoke guest sessions")
		}

		resolved, err := model.NewServiceRequestModel(s).ResolveOpenForSession(ctx, closed.ID)
		if err != nil {
			l.Errorf("resolve open service requests for %s: %v", closed.ID, err)
			return apierr.Internal("resolve open service requests")
		}
		for i := range resolved {
			r := &resolved[i]
			if err := outbox.Insert(ctx, eventServiceRequestTransited, closed.VenueID,
				model.TopicServiceRequest, r.TableID,
				serviceRequestTransitionedPayload{
					ServiceRequestID: r.ID,
					TableID:          r.TableID,
					TableSessionID:   r.TableSessionID,
					Type:             r.Type,
					From:             model.RequestOpen,
					To:               model.RequestResolved,
					ActorStaffID:     in.GetActorStaffId(),
				}, traceID(ctx)); err != nil {
				l.Errorf("write service_request.transitioned outbox row: %v", err)
				return apierr.Internal("write outbox row")
			}
		}

		return outbox.Insert(ctx, eventTableSessionClosed, closed.VenueID, model.TopicOrder, closed.TableID,
			tableSessionClosedPayload{
				TableSessionID: closed.ID,
				TableID:        closed.TableID,
				TotalMinor:     closed.TotalMinor,
				Currency:       closed.Currency,
				PaymentMethod:  method,
				ActorStaffID:   in.GetActorStaffId(),
			}, traceID(ctx))
	})
	if err != nil {
		return nil, err
	}
	return tableSessionToProto(closed), nil
}

// explainCloseFailure distinguishes "no such session" from "already
// closed". Both make the conditional UPDATE affect zero rows, but they
// mean very different things to the person holding the card machine.
func (l *CloseTableSessionLogic) explainCloseFailure(ctx context.Context, s sqlx.Session, in *v1_orderpb.CloseTableSessionRequest) error {
	existing, err := model.NewTableSessionModel(s).FindByID(ctx, in.GetVenueId(), in.GetTableSessionId())
	if err != nil {
		if isNotFound(err) {
			return apierr.NotFound("table session not found")
		}
		l.Errorf("look up table session %s: %v", in.GetTableSessionId(), err)
		return apierr.Internal("look up table session")
	}
	if existing.Status == model.TableSessionClosed {
		return apierr.SessionClosed("table session is already closed")
	}
	return apierr.Internal("close table session")
}
