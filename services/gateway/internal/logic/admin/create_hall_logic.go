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

type CreateHallLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewCreateHallLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateHallLogic {
	return &CreateHallLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *CreateHallLogic) CreateHall(req *types.CreateHallReq) (resp *types.Hall, err error) {
	claims, err := authz.Admin(l.ctx)
	if err != nil {
		return nil, err
	}
	if req.Name == "" {
		return nil, errs.New(errs.CodeValidationFailed, "name is required")
	}

	hall, err := l.svcCtx.CatalogRpc.CreateHall(l.ctx, &v1_catalogpb.CreateHallRequest{
		VenueId:   claims.VenueID,
		Name:      req.Name,
		SortOrder: req.SortOrder,
	})
	if err != nil {
		return nil, rpcerr.FromCatalog(err)
	}

	out := convert.Hall(hall)
	return &out, nil
}
