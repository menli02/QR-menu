// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package admin

import (
	"context"

	v1_identitypb "github.com/menli02/QR-menu/proto/identity/v1"
	"github.com/menli02/QR-menu/services/gateway/internal/authz"
	"github.com/menli02/QR-menu/services/gateway/internal/errs"
	"github.com/menli02/QR-menu/services/gateway/internal/rpcerr"
	"github.com/menli02/QR-menu/services/gateway/internal/svc"
	"github.com/menli02/QR-menu/services/gateway/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type DeleteStaffLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewDeleteStaffLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DeleteStaffLogic {
	return &DeleteStaffLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// DeleteStaff deactivates an account (identity soft-deletes: other
// services hold staff_id as an opaque reference on historical orders and
// audit events, and a hard delete would strand them).
func (l *DeleteStaffLogic) DeleteStaff(req *types.DeleteStaffReq) (resp *types.DeletedResp, err error) {
	claims, err := authz.Admin(l.ctx)
	if err != nil {
		return nil, err
	}
	if req.Id == "" {
		return nil, errs.New(errs.CodeValidationFailed, "staff id is required")
	}
	if req.Id == claims.StaffID {
		return nil, errs.New(errs.CodeValidationFailed, "you cannot remove your own account")
	}

	deleted, err := l.svcCtx.IdentityRpc.DeleteStaff(l.ctx, &v1_identitypb.DeleteStaffRequest{
		VenueId:      claims.VenueID,
		StaffId:      req.Id,
		ActorStaffId: claims.StaffID,
	})
	if err != nil {
		return nil, rpcerr.From(err)
	}

	return &types.DeletedResp{Deleted: deleted.GetDeleted()}, nil
}
