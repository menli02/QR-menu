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

const (
	defaultTablePageSize = 50
	maxTablePageSize     = 200
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
	cursor, err := model.DecodeCursor(in.GetCursor())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid cursor: %v", err)
	}
	limit := int(in.GetPageSize())
	if limit <= 0 || limit > maxTablePageSize {
		limit = defaultTablePageSize
	}

	rows, err := model.NewTableModel(l.svcCtx.DB).List(l.ctx, in.GetVenueId(), in.GetHallId(), cursor, limit)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list tables: %v", err)
	}

	resp := &v1_catalogpb.ListTablesResponse{Tables: make([]*v1_catalogpb.Table, len(rows))}
	for i := range rows {
		resp.Tables[i] = tableToProto(&rows[i])
	}
	if len(rows) == limit {
		last := rows[len(rows)-1]
		resp.NextCursor = model.Cursor{CreatedAt: last.CreatedAt, ID: last.ID}.Encode()
	}
	return resp, nil
}
