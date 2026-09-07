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

type RotateVenueQrKeyLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewRotateVenueQrKeyLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RotateVenueQrKeyLogic {
	return &RotateVenueQrKeyLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// RotateVenueQrKey issues a new QR signing key (FR-T4). Codes printed
// under the previous key keep working for the grace period, so a venue can
// reprint at its own pace instead of having every table go dark at once.
//
// The grace period is left at catalog's default (30 days); the public
// request has no field for it.
func (l *RotateVenueQrKeyLogic) RotateVenueQrKey() (resp *types.RotateVenueQrKeyResp, err error) {
	claims, err := authz.Admin(l.ctx)
	if err != nil {
		return nil, err
	}

	rotated, err := l.svcCtx.CatalogRpc.RotateVenueQRKey(l.ctx, &v1_catalogpb.RotateVenueQRKeyRequest{
		VenueId:      claims.VenueID,
		ActorStaffId: claims.StaffID,
	})
	if err != nil {
		return nil, rpcerr.FromCatalog(err)
	}

	return &types.RotateVenueQrKeyResp{
		NewKeyVersion:        rotated.GetNewKeyVersion(),
		PreviousKeyVersion:   rotated.GetPreviousKeyVersion(),
		PreviousKeyExpiresAt: convert.Time(rotated.GetPreviousKeyExpiresAt()),
	}, nil
}
