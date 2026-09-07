// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package admin

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

type ListTablesLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewListTablesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListTablesLogic {
	return &ListTablesLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ListTablesLogic) ListTables(req *types.ListTablesReq) (resp *types.ListTablesResp, err error) {
	claims, err := authz.Admin(l.ctx)
	if err != nil {
		return nil, err
	}

	tables, err := l.svcCtx.CatalogRpc.ListTables(l.ctx, &v1_catalogpb.ListTablesRequest{
		VenueId:  claims.VenueID,
		HallId:   req.HallId,
		Cursor:   req.Cursor,
		PageSize: req.PageSize,
	})
	if err != nil {
		return nil, rpcerr.FromCatalog(err)
	}

	return &types.ListTablesResp{
		Tables:     convert.Tables(tables.GetTables()),
		NextCursor: tables.GetNextCursor(),
	}, nil
}
