package catalogservicelogic

import (
	"context"

	"github.com/menli02/QR-menu/proto/catalog/v1"
	"github.com/menli02/QR-menu/services/catalog/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type SetItemAvailabilityLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewSetItemAvailabilityLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SetItemAvailabilityLogic {
	return &SetItemAvailabilityLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// SetItemAvailability toggles the stop-list ("86") state of a menu item.
func (l *SetItemAvailabilityLogic) SetItemAvailability(in *v1_catalogpb.SetItemAvailabilityRequest) (*v1_catalogpb.SetItemAvailabilityResponse, error) {
	// todo: add your logic here and delete this line

	return &v1_catalogpb.SetItemAvailabilityResponse{}, nil
}
