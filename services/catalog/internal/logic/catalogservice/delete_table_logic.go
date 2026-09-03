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

type DeleteTableLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewDeleteTableLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DeleteTableLogic {
	return &DeleteTableLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// DeleteTable implements FR-T5 exactly: "Deactivating a table blocks new
// orders with a clear message and keeps history."
func (l *DeleteTableLogic) DeleteTable(in *v1_catalogpb.DeleteTableRequest) (*v1_catalogpb.DeleteTableResponse, error) {
	deleted, err := model.NewTableModel(l.svcCtx.DB).Deactivate(l.ctx, in.GetTableId(), in.GetVenueId())
	if err != nil {
		return nil, status.Errorf(codes.Internal, "deactivate table: %v", err)
	}
	return &v1_catalogpb.DeleteTableResponse{Deleted: deleted}, nil
}
