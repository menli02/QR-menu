// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package floor

import (
	"context"

	v1_catalogpb "github.com/menli02/QR-menu/proto/catalog/v1"
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

// ListFloorTables is the table map (§8.1 GET /floor/tables).
//
// Known gap, stated rather than papered over: §8.1 describes this as "table
// map with session state and totals", but the public Table type in
// gateway.api carries no session fields — no status, no total, no session
// id. So this returns the tables only, and the floor UI has to correlate
// them with ListActiveTableSessions itself.
//
// Fixing it properly means either an additive FloorTable type here or a
// dedicated RPC that joins the two, and both are contract changes that
// should be decided rather than slipped in. What is *not* acceptable is
// inventing a shape the spec doesn't describe and having the frontend
// build against it.
//
// Note also that tables and sessions live in different services (catalog
// and order), with no cross-service join available — which is precisely
// why the correlation has to happen in a client or in a purpose-built
// response type.
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

	return &types.ListFloorTablesResp{Tables: convert.Tables(tables.GetTables())}, nil
}
