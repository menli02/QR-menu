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

type GetVenueSettingsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetVenueSettingsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetVenueSettingsLogic {
	return &GetVenueSettingsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// GetVenueSettings returns the venue's configuration (FR-A2).
//
// The venue is taken from the caller's token, so there is no id to
// substitute: an admin of one venue cannot read another's settings.
func (l *GetVenueSettingsLogic) GetVenueSettings() (resp *types.VenueSettings, err error) {
	claims, err := authz.Admin(l.ctx)
	if err != nil {
		return nil, err
	}

	settings, err := l.svcCtx.CatalogRpc.GetVenueSettings(l.ctx, &v1_catalogpb.GetVenueSettingsRequest{
		VenueId: claims.VenueID,
	})
	if err != nil {
		return nil, rpcerr.FromCatalog(err)
	}

	out := convert.VenueSettings(settings)
	return &out, nil
}
