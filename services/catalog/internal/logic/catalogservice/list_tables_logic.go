package catalogservicelogic

import (
	"context"

	"github.com/menli02/QR-menu/proto/catalog/v1"
	"github.com/menli02/QR-menu/services/catalog/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListTablesLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListTablesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListTablesLogic {
	return &ListTablesLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ListTablesLogic) ListTables(in *v1_catalogpb.ListTablesRequest) (*v1_catalogpb.ListTablesResponse, error) {
	// todo: add your logic here and delete this line

	return &v1_catalogpb.ListTablesResponse{}, nil
}
