// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package admin

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

type UpdateHallLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewUpdateHallLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateHallLogic {
	return &UpdateHallLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *UpdateHallLogic) UpdateHall(req *types.UpdateHallReq) (resp *types.Hall, err error) {
	claims, err := authz.Admin(l.ctx)
	if err != nil {
		return nil, err
	}
	if req.Id == "" || req.Name == "" {
		return nil, errs.New(errs.CodeValidationFailed, "hall id and name are required")
	}

	hall, err := l.svcCtx.CatalogRpc.UpdateHall(l.ctx, &v1_catalogpb.UpdateHallRequest{
		Hall: &v1_catalogpb.Hall{
			Id:        req.Id,
			VenueId:   claims.VenueID,
			Name:      req.Name,
			SortOrder: req.SortOrder,
			IsActive:  req.IsActive,
		},
	})
	if err != nil {
		return nil, rpcerr.FromCatalog(err)
	}

	out := convert.Hall(hall)
	return &out, nil
}
