// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package floor

import (
	"context"

	v1_catalogpb "github.com/menli02/QR-menu/proto/catalog/v1"
	v1_orderpb "github.com/menli02/QR-menu/proto/order/v1"
	"github.com/menli02/QR-menu/services/gateway/internal/authz"
	"github.com/menli02/QR-menu/services/gateway/internal/convert"
	"github.com/menli02/QR-menu/services/gateway/internal/rpcerr"
	"github.com/menli02/QR-menu/services/gateway/internal/svc"
	"github.com/menli02/QR-menu/services/gateway/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

// floorTablePageSize bounds the table map. A venue has tens of tables, not
// thousands, and the floor view wants them all at once — but an unbounded
// fetch is still an unbounded fetch, so it pages defensively.
const floorTablePageSize = 500

type ListFloorTablesLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewListFloorTablesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListFloorTablesLogic {
	return &ListFloorTablesLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// ListFloorTables is the table map (§8.1 GET /floor/tables): every table
// with whatever is happening at it.
//
// This is the one route that genuinely needs the gateway to be a BFF.
// Tables belong to catalog and sessions to order, there is no cross-service
// join (docs/TZ.md §6 forbids even a foreign key between them), and a floor
// view that made the client fetch both and correlate them would push the
// same loop into three frontends.
//
// Two calls, not N+1: the venue's tables and the venue's *open* sessions,
// joined in memory on table_id. Asking order for a session per table would
// be one RPC per table on a screen that refreshes constantly.
func (l *ListFloorTablesLogic) ListFloorTables() (resp *types.ListFloorTablesResp, err error) {
	claims, err := authz.Staff(l.ctx)
	if err != nil {
		return nil, err
	}

	tables, err := l.svcCtx.CatalogRpc.ListTables(l.ctx, &v1_catalogpb.ListTablesRequest{
		VenueId:  claims.VenueID,
		PageSize: floorTablePageSize,
	})
	if err != nil {
		return nil, rpcerr.FromCatalog(err)
	}

	sessions, err := l.svcCtx.OrderRpc.ListActiveTableSessions(l.ctx, &v1_orderpb.ListActiveTableSessionsRequest{
		VenueId: claims.VenueID,
	})
	if err != nil {
		return nil, rpcerr.From(err)
	}

	// order guarantees at most one open session per table (a partial
	// unique index enforces it), so keying by table_id cannot lose one.
	byTable := make(map[string]*v1_orderpb.TableSession, len(sessions.GetSessions()))
	for _, s := range sessions.GetSessions() {
		byTable[s.GetTableId()] = s
	}

	out := &types.ListFloorTablesResp{
		Tables: make([]types.FloorTable, 0, len(tables.GetTables())),
	}
	for _, t := range tables.GetTables() {
		ft := types.FloorTable{
			Id:       t.GetId(),
			HallId:   t.GetHallId(),
			Label:    t.GetLabel(),
			Seats:    t.GetSeats(),
			IsActive: t.GetIsActive(),
		}
		// table_code and key_version are deliberately not carried over
		// from the catalog Table. The floor view is a seating map; the
		// code is print material, and there is no reason for it to be on
		// a screen a waiter carries around a dining room.
		if s, ok := byTable[t.GetId()]; ok {
			ft.Occupied = true
			ft.TableSessionId = s.GetId()
			ft.OpenedAt = convert.Time(s.GetOpenedAt())
			ft.TotalMinor = s.GetTotalMinor()
			ft.Currency = s.GetCurrency()
		}
		out.Tables = append(out.Tables, ft)
	}
	return out, nil
}
