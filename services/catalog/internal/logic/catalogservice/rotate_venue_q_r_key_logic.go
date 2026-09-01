package catalogservicelogic

import (
	"context"

	"github.com/menli02/QR-menu/proto/catalog/v1"
	"github.com/menli02/QR-menu/services/catalog/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type RotateVenueQRKeyLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewRotateVenueQRKeyLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RotateVenueQRKeyLogic {
	return &RotateVenueQRKeyLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// RotateVenueQRKey issues a new HMAC key_version for QR signing; the
func (l *RotateVenueQRKeyLogic) RotateVenueQRKey(in *v1_catalogpb.RotateVenueQRKeyRequest) (*v1_catalogpb.RotateVenueQRKeyResponse, error) {
	// todo: add your logic here and delete this line

	return &v1_catalogpb.RotateVenueQRKeyResponse{}, nil
}
