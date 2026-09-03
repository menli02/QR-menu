package catalogservicelogic

import (
	"context"
	"errors"
	"time"

	"github.com/menli02/QR-menu/proto/catalog/v1"
	"github.com/menli02/QR-menu/services/catalog/internal/model"
	"github.com/menli02/QR-menu/services/catalog/internal/qrsig"
	"github.com/menli02/QR-menu/services/catalog/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type ResolveTableLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewResolveTableLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ResolveTableLogic {
	return &ResolveTableLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// ResolveTable validates a QR link's signature (FR-T2, FR-T4). venue_id/
// table_id/hall_id/table_label are still populated even when sig_valid
// is false — this is an internal call from gateway (a trusted caller,
// over private gRPC), which decides what to expose to the actual guest;
// gateway must reject the request outright when sig_valid is false, not
// mint a token regardless (R1: signed table token + rate limits are the
// primary defense against QR cloning, docs/TZ.md R1 in §1.1).
//
// venue_slug / table_code not resolving to anything at all (unlike a bad
// signature on a real one) is reported as NotFound — there is nothing to
// return partial fields about.
func (l *ResolveTableLogic) ResolveTable(in *v1_catalogpb.ResolveTableRequest) (*v1_catalogpb.ResolveTableResponse, error) {
	venue, err := model.NewVenueModel(l.svcCtx.DB).FindBySlug(l.ctx, in.GetVenueSlug())
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			return nil, status.Error(codes.NotFound, "venue not found")
		}
		return nil, status.Errorf(codes.Internal, "look up venue: %v", err)
	}

	table, err := model.NewTableModel(l.svcCtx.DB).FindByVenueAndCode(l.ctx, venue.ID, in.GetTableCode())
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			return nil, status.Error(codes.NotFound, "table not found")
		}
		return nil, status.Errorf(codes.Internal, "look up table: %v", err)
	}

	resp := &v1_catalogpb.ResolveTableResponse{
		VenueId:       venue.ID,
		TableId:       table.ID,
		HallId:        table.HallID,
		TableLabel:    table.Label,
		TableIsActive: table.IsActive,
	}

	key, err := model.NewVenueQRKeyModel(l.svcCtx.DB).FindByVersion(l.ctx, venue.ID, in.GetKeyVersion())
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			// An unknown key_version can't be verified — sig_valid stays
			// false, but this is still a normal "bad/stale QR" response,
			// not a server error.
			return resp, nil
		}
		return nil, status.Errorf(codes.Internal, "look up QR key: %v", err)
	}

	resp.SigValid = qrsig.Verify(key.Secret, venue.ID, table.TableCode, in.GetSig())
	resp.KeyVersionExpired = key.ExpiresAt.Valid && key.ExpiresAt.Time.Before(time.Now())
	return resp, nil
}
