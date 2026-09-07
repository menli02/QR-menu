package orderservicelogic

import (
	"context"

	"github.com/menli02/QR-menu/proto/order/v1"
	"github.com/menli02/QR-menu/services/order/internal/apierr"
	"github.com/menli02/QR-menu/services/order/internal/model"
	"github.com/menli02/QR-menu/services/order/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetTableSessionLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetTableSessionLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetTableSessionLogic {
	return &GetTableSessionLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// GetTableSession returns the *open* session at a table, if any.
//
// The request keys on table_id, not session id, so this answers "what is
// happening at this table right now" — the question the floor view and a
// returning guest both ask. A table with no open session is NOT_FOUND
// rather than an empty session: there is genuinely nothing there, and a
// zero-valued TableSession would be indistinguishable from a real one
// with an empty bill.
func (l *GetTableSessionLogic) GetTableSession(in *v1_orderpb.GetTableSessionRequest) (*v1_orderpb.TableSession, error) {
	if in.GetVenueId() == "" || in.GetTableId() == "" {
		return nil, apierr.Validation("venue_id and table_id are required")
	}

	session, err := model.NewTableSessionModel(l.svcCtx.DB).FindOpenByTable(l.ctx, in.GetVenueId(), in.GetTableId())
	if err != nil {
		if isNotFound(err) {
			return nil, apierr.NotFound("no open session at this table")
		}
		l.Errorf("look up open session for table %s: %v", in.GetTableId(), err)
		return nil, apierr.Internal("look up table session")
	}
	return tableSessionToProto(session), nil
}
