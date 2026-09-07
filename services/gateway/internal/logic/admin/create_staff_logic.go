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

type CreateStaffLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewCreateStaffLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateStaffLogic {
	return &CreateStaffLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// minPasswordLength is a floor, not a policy. docs/TZ.md FR-A1 defers the
// real password rules to §11.3, which is not written — so this rejects the
// obviously unacceptable and leaves the rest to identity, which does the
// hashing and is where a policy belongs.
const minPasswordLength = 12

func (l *CreateStaffLogic) CreateStaff(req *types.CreateStaffReq) (resp *types.Staff, err error) {
	claims, err := authz.Admin(l.ctx)
	if err != nil {
		return nil, err
	}
	if req.Name == "" || req.Email == "" {
		return nil, errs.New(errs.CodeValidationFailed, "name and email are required")
	}
	if len(req.Password) < minPasswordLength {
		return nil, errs.New(errs.CodeValidationFailed, "password must be at least 12 characters")
	}
	role, ok := convert.Roles[req.Role]
	if !ok {
		return nil, errs.New(errs.CodeValidationFailed, "role must be admin, manager, waiter or cook")
	}

	staff, err := l.svcCtx.IdentityRpc.CreateStaff(l.ctx, &v1_identitypb.CreateStaffRequest{
		VenueId:      claims.VenueID,
		Name:         req.Name,
		Email:        req.Email,
		Password:     req.Password,
		Role:         role,
		ActorStaffId: claims.StaffID,
	})
	if err != nil {
		return nil, rpcerr.From(err)
	}

	out := convert.Staff(staff)
	return &out, nil
}
