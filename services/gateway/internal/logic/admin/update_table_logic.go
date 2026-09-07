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

type UpdateTableLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewUpdateTableLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateTableLogic {
	return &UpdateTableLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *UpdateTableLogic) UpdateTable(req *types.UpdateTableReq) (resp *types.Table, err error) {
	claims, err := authz.Admin(l.ctx)
	if err != nil {
		return nil, err
	}
	if req.Id == "" || req.HallId == "" || req.Label == "" {
		return nil, errs.New(errs.CodeValidationFailed, "table id, hallId and label are required")
	}

	// table_code and key_version are intentionally not sent: they are set
	// when the table is created and changed only by a QR key rotation.
	// Letting an admin edit form overwrite them would invalidate every
	// sticker already on that table.
	table, err := l.svcCtx.CatalogRpc.UpdateTable(l.ctx, &v1_catalogpb.UpdateTableRequest{
		Table: &v1_catalogpb.Table{
			Id:       req.Id,
			VenueId:  claims.VenueID,
			HallId:   req.HallId,
			Label:    req.Label,
			Seats:    req.Seats,
			IsActive: req.IsActive,
		},
	})
	if err != nil {
		return nil, rpcerr.FromCatalog(err)
	}

	out := convert.Table(table)
	return &out, nil
}
