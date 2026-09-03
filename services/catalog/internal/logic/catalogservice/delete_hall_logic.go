package catalogservicelogic

import (
	"context"

	"github.com/menli02/QR-menu/proto/catalog/v1"
	"github.com/menli02/QR-menu/services/catalog/internal/model"
	"github.com/menli02/QR-menu/services/catalog/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type DeleteHallLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewDeleteHallLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DeleteHallLogic {
	return &DeleteHallLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// DeleteHall soft-deletes (is_active = false) — see HallModel.Deactivate's
// comment for why: tables.hall_id has no ON DELETE CASCADE, so a hard
// delete of a hall with any tables would fail the foreign key regardless.
func (l *DeleteHallLogic) DeleteHall(in *v1_catalogpb.DeleteHallRequest) (*v1_catalogpb.DeleteHallResponse, error) {
	deleted, err := model.NewHallModel(l.svcCtx.DB).Deactivate(l.ctx, in.GetHallId(), in.GetVenueId())
	if err != nil {
		return nil, status.Errorf(codes.Internal, "deactivate hall: %v", err)
	}
	return &v1_catalogpb.DeleteHallResponse{Deleted: deleted}, nil
}
