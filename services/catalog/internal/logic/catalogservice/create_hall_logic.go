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

type CreateHallLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCreateHallLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateHallLogic {
	return &CreateHallLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *CreateHallLogic) CreateHall(in *v1_catalogpb.CreateHallRequest) (*v1_catalogpb.Hall, error) {
	if in.GetName() == "" {
		return nil, status.Error(codes.InvalidArgument, "name is required")
	}
	h, err := model.NewHallModel(l.svcCtx.DB).Insert(l.ctx, in.GetVenueId(), in.GetName(), in.GetSortOrder())
	if err != nil {
		return nil, status.Errorf(codes.Internal, "create hall: %v", err)
	}
	return hallToProto(h), nil
}
