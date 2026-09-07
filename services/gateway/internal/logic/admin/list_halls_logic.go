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

type ListHallsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewListHallsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListHallsLogic {
	return &ListHallsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ListHallsLogic) ListHalls() (resp *types.ListHallsResp, err error) {
	claims, err := authz.Admin(l.ctx)
	if err != nil {
		return nil, err
	}

	halls, err := l.svcCtx.CatalogRpc.ListHalls(l.ctx, &v1_catalogpb.ListHallsRequest{VenueId: claims.VenueID})
	if err != nil {
		return nil, rpcerr.FromCatalog(err)
	}

	return &types.ListHallsResp{Halls: convert.Halls(halls.GetHalls())}, nil
}
