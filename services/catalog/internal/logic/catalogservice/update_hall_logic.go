package catalogservicelogic

import (
	"context"
	"errors"

	"github.com/menli02/QR-menu/proto/catalog/v1"
	"github.com/menli02/QR-menu/services/catalog/internal/model"
	"github.com/menli02/QR-menu/services/catalog/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type UpdateHallLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewUpdateHallLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateHallLogic {
	return &UpdateHallLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *UpdateHallLogic) UpdateHall(in *v1_catalogpb.UpdateHallRequest) (*v1_catalogpb.Hall, error) {
	h := in.GetHall()
	if h == nil || h.GetName() == "" {
		return nil, status.Error(codes.InvalidArgument, "hall with a name is required")
	}
	updated, err := model.NewHallModel(l.svcCtx.DB).Update(l.ctx, h.GetId(), h.GetVenueId(), h.GetName(), h.GetSortOrder(), h.GetIsActive())
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			return nil, status.Error(codes.NotFound, "hall not found")
		}
		return nil, status.Errorf(codes.Internal, "update hall: %v", err)
	}
	return hallToProto(updated), nil
}
