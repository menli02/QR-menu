// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package admin

import (
	"context"

	v1_identitypb "github.com/menli02/QR-menu/proto/identity/v1"
	"github.com/menli02/QR-menu/services/gateway/internal/authz"
	"github.com/menli02/QR-menu/services/gateway/internal/convert"
	"github.com/menli02/QR-menu/services/gateway/internal/rpcerr"
	"github.com/menli02/QR-menu/services/gateway/internal/svc"
	"github.com/menli02/QR-menu/services/gateway/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListStaffLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewListStaffLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListStaffLogic {
	return &ListStaffLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ListStaffLogic) ListStaff(req *types.ListStaffReq) (resp *types.ListStaffResp, err error) {
	claims, err := authz.Admin(l.ctx)
	if err != nil {
		return nil, err
	}

	list, err := l.svcCtx.IdentityRpc.ListStaff(l.ctx, &v1_identitypb.ListStaffRequest{
		VenueId:  claims.VenueID,
		Cursor:   req.Cursor,
		PageSize: req.PageSize,
	})
	if err != nil {
		return nil, rpcerr.From(err)
	}

	return &types.ListStaffResp{
		Staff:      convert.StaffList(list.GetStaff()),
		NextCursor: list.GetNextCursor(),
	}, nil
}
