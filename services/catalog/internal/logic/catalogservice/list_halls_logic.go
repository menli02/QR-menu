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

type ListHallsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListHallsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListHallsLogic {
	return &ListHallsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ListHallsLogic) ListHalls(in *v1_catalogpb.ListHallsRequest) (*v1_catalogpb.ListHallsResponse, error) {
	halls, err := model.NewHallModel(l.svcCtx.DB).List(l.ctx, in.GetVenueId())
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list halls: %v", err)
	}
	resp := &v1_catalogpb.ListHallsResponse{Halls: make([]*v1_catalogpb.Hall, len(halls))}
	for i := range halls {
		resp.Halls[i] = hallToProto(&halls[i])
	}
	return resp, nil
}
