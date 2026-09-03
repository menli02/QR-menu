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

type UpdateTableLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewUpdateTableLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateTableLogic {
	return &UpdateTableLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// UpdateTable changes hall/label/seats/is_active only — table_code and
// key_version are immutable once assigned (FR-T2); see TableModel.Insert's
// comment.
func (l *UpdateTableLogic) UpdateTable(in *v1_catalogpb.UpdateTableRequest) (*v1_catalogpb.Table, error) {
	t := in.GetTable()
	if t == nil || t.GetLabel() == "" || t.GetHallId() == "" {
		return nil, status.Error(codes.InvalidArgument, "table with a label and hall_id is required")
	}
	seats := t.GetSeats()
	if seats <= 0 {
		seats = 1
	}

	updated, err := model.NewTableModel(l.svcCtx.DB).Update(l.ctx, t.GetId(), t.GetVenueId(), t.GetHallId(), t.GetLabel(), seats, t.GetIsActive())
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			return nil, status.Error(codes.NotFound, "table not found")
		}
		return nil, status.Errorf(codes.Internal, "update table: %v", err)
	}
	return tableToProto(updated), nil
}
