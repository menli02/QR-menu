// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package admin

import (
	"context"

	v1_identitypb "github.com/menli02/QR-menu/proto/identity/v1"
	"github.com/menli02/QR-menu/services/gateway/internal/authz"
	"github.com/menli02/QR-menu/services/gateway/internal/convert"
	"github.com/menli02/QR-menu/services/gateway/internal/errs"
	"github.com/menli02/QR-menu/services/gateway/internal/rpcerr"
	"github.com/menli02/QR-menu/services/gateway/internal/svc"
	"github.com/menli02/QR-menu/services/gateway/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type UpdateStaffLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewUpdateStaffLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateStaffLogic {
	return &UpdateStaffLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// UpdateStaff edits a colleague's name, email, role and active flag.
//
// Password is not editable here — identity has SetStaffPassword for that,
// and §8.1 exposes no route to it. Folding a password into a general
// profile form would mean it travels on every unrelated edit.
//
// An admin cannot deactivate their own account: locking the last admin out
// of a venue is unrecoverable without operator intervention, and this
// check is the cheap half of preventing it. The other half — refusing to
// demote or remove the last remaining admin — needs a count identity does
// not expose, and is flagged rather than half-implemented.
func (l *UpdateStaffLogic) UpdateStaff(req *types.UpdateStaffReq) (resp *types.Staff, err error) {
	claims, err := authz.Admin(l.ctx)
	if err != nil {
		return nil, err
	}
	if req.Id == "" || req.Name == "" || req.Email == "" {
		return nil, errs.New(errs.CodeValidationFailed, "staff id, name and email are required")
	}
	role, ok := convert.Roles[req.Role]
	if !ok {
		return nil, errs.New(errs.CodeValidationFailed, "role must be admin, manager, waiter or cook")
	}
	if req.Id == claims.StaffID && !req.IsActive {
		return nil, errs.New(errs.CodeValidationFailed, "you cannot deactivate your own account")
	}

	staff, err := l.svcCtx.IdentityRpc.UpdateStaff(l.ctx, &v1_identitypb.UpdateStaffRequest{
		Staff: &v1_identitypb.Staff{
			Id:       req.Id,
			VenueId:  claims.VenueID,
			Name:     req.Name,
			Email:    req.Email,
			Role:     role,
			IsActive: req.IsActive,
		},
		ActorStaffId: claims.StaffID,
	})
	if err != nil {
		return nil, rpcerr.From(err)
	}

	out := convert.Staff(staff)
	return &out, nil
}
