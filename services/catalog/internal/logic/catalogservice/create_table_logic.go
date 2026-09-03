package catalogservicelogic

import (
	"context"
	"errors"

	"github.com/menli02/QR-menu/proto/catalog/v1"
	"github.com/menli02/QR-menu/services/catalog/internal/model"
	"github.com/menli02/QR-menu/services/catalog/internal/qrsig"
	"github.com/menli02/QR-menu/services/catalog/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type CreateTableLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCreateTableLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateTableLogic {
	return &CreateTableLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// CreateTable assigns a fresh table_code and signs it under the venue's
// current QR key (FR-T2). The signature itself isn't returned or stored —
// it's cheaply recomputed on demand from (secret, venue_id, table_code)
// whenever it's needed (ResolveTable does exactly that). NOTE: nothing in
// the current proto contract exposes a way to RETRIEVE that signature for
// printing a QR code (Table has no `sig` field, and there's no dedicated
// RPC for it) — a real gap, flagged rather than silently patched around
// with an unrequested proto change; low urgency today since gateway's QR
// export endpoint is itself still a 501 stub.
func (l *CreateTableLogic) CreateTable(in *v1_catalogpb.CreateTableRequest) (*v1_catalogpb.Table, error) {
	if in.GetLabel() == "" {
		return nil, status.Error(codes.InvalidArgument, "label is required")
	}
	if in.GetHallId() == "" {
		return nil, status.Error(codes.InvalidArgument, "hall_id is required")
	}

	currentKey, err := model.NewVenueQRKeyModel(l.svcCtx.DB).FindCurrent(l.ctx, in.GetVenueId())
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			return nil, status.Error(codes.FailedPrecondition, "venue has no QR signing key provisioned")
		}
		return nil, status.Errorf(codes.Internal, "look up current QR key: %v", err)
	}

	tableCode, err := qrsig.GenerateTableCode()
	if err != nil {
		return nil, status.Errorf(codes.Internal, "generate table code: %v", err)
	}

	seats := in.GetSeats()
	if seats <= 0 {
		seats = 1
	}

	t, err := model.NewTableModel(l.svcCtx.DB).Insert(l.ctx, in.GetVenueId(), in.GetHallId(), in.GetLabel(), seats, tableCode, currentKey.KeyVersion)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "create table: %v", err)
	}
	return tableToProto(t), nil
}
