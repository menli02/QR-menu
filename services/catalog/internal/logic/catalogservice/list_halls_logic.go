package catalogservicelogic

import (
	"context"

	"github.com/menli02/QR-menu/proto/catalog/v1"
	"github.com/menli02/QR-menu/services/catalog/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
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
	// todo: add your logic here and delete this line

	return &v1_catalogpb.ListHallsResponse{}, nil
}
