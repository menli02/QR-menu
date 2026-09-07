// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package kds

import (
	"context"

	v1_catalogpb "github.com/menli02/QR-menu/proto/catalog/v1"
	"github.com/menli02/QR-menu/services/gateway/internal/authz"
	"github.com/menli02/QR-menu/services/gateway/internal/convert"
	"github.com/menli02/QR-menu/services/gateway/internal/errs"
	"github.com/menli02/QR-menu/services/gateway/internal/rpcerr"
	"github.com/menli02/QR-menu/services/gateway/internal/svc"
	"github.com/menli02/QR-menu/services/gateway/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type SetItemAvailabilityLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewSetItemAvailabilityLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SetItemAvailabilityLogic {
	return &SetItemAvailabilityLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// SetItemAvailability is the "86 this" stop-list toggle (FR-C4, FR-K6),
// reachable straight from a kitchen ticket.
//
// Any staff role may do it, deliberately: the cook who has just run out of
// an ingredient is exactly the person who should be able to take it off
// the menu, and making them find a manager is how a venue ends up serving
// orders it cannot fulfil.
//
// No idempotency key: the write is an absolute set, not a delta, so
// replaying it is harmless by construction.
func (l *SetItemAvailabilityLogic) SetItemAvailability(req *types.SetItemAvailabilityReq) (resp *types.MenuItem, err error) {
	claims, err := authz.Staff(l.ctx)
	if err != nil {
		return nil, err
	}
	if req.ItemId == "" {
		return nil, errs.New(errs.CodeValidationFailed, "item id is required")
	}

	updated, err := l.svcCtx.CatalogRpc.SetItemAvailability(l.ctx, &v1_catalogpb.SetItemAvailabilityRequest{
		VenueId:      claims.VenueID,
		ItemId:       req.ItemId,
		IsAvailable:  req.IsAvailable,
		ActorStaffId: claims.StaffID,
	})
	if err != nil {
		return nil, rpcerr.FromCatalog(err)
	}

	out := convert.MenuItem(updated.GetItem(), "", "")
	return &out, nil
}
