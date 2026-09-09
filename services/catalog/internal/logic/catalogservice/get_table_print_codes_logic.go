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

// printCodePageSize bounds one sweep of a venue's tables. A venue has tens
// of tables; this is a ceiling against a pathological row count, not a
// pagination scheme — the caller wants the whole set to print at once.
const printCodePageSize = 1000

type GetTablePrintCodesLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetTablePrintCodesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetTablePrintCodesLogic {
	return &GetTablePrintCodesLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// GetTablePrintCodes returns each table's signed QR payload (FR-T3).
//
// This is the only RPC in the service that emits signing output, and the
// value it emits is a credential: venue_slug + table_code + sig is exactly
// what POST /guest/sessions accepts to open a session at that table. Three
// consequences shape the implementation.
//
// First, it signs with each table's *own* key_version rather than the
// venue's current key. A table printed before a rotation still carries the
// old version in its sticker, and re-signing it under the new key would
// produce a code that verifies against nothing the guest has.
//
// Second, inactive tables are skipped when the caller asks for "all". A
// sticker for a table that is out of service is at best wasted paper and
// at worst a way for a guest to open a session at a table nobody is
// watching. An explicitly requested id is still returned — the caller
// named it, and printing a specific table before bringing it back into
// service is a real thing to want.
//
// Third, it logs who asked. That is the point of the actor field: an
// unexplained bulk export of a venue's QR credentials is something an
// operator should be able to find afterwards.
func (l *GetTablePrintCodesLogic) GetTablePrintCodes(in *v1_catalogpb.GetTablePrintCodesRequest) (*v1_catalogpb.GetTablePrintCodesResponse, error) {
	if in.GetVenueId() == "" {
		return nil, status.Error(codes.InvalidArgument, "venue_id is required")
	}

	venue, err := model.NewVenueModel(l.svcCtx.DB).FindByID(l.ctx, in.GetVenueId())
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			return nil, status.Error(codes.NotFound, "venue not found")
		}
		l.Errorf("look up venue %s: %v", in.GetVenueId(), err)
		return nil, status.Error(codes.Internal, "look up venue")
	}

	tables, err := l.selectTables(in)
	if err != nil {
		return nil, err
	}

	// One lookup per distinct key_version, not per table: a venue that has
	// never rotated needs exactly one, and even a heavily rotated one has
	// a handful.
	keyModel := model.NewVenueQRKeyModel(l.svcCtx.DB)
	secrets := make(map[int32][]byte)

	resp := &v1_catalogpb.GetTablePrintCodesResponse{
		VenueSlug: venue.Slug,
		Codes:     make([]*v1_catalogpb.TablePrintCode, 0, len(tables)),
	}

	for i := range tables {
		t := &tables[i]
		secret, ok := secrets[t.KeyVersion]
		if !ok {
			key, err := keyModel.FindByVersion(l.ctx, in.GetVenueId(), t.KeyVersion)
			if err != nil {
				if errors.Is(err, model.ErrNotFound) {
					// The table references a key that no longer exists.
					// Skipping beats emitting a code that would fail
					// verification and send a guest to a support desk.
					l.Errorf("table %s references missing key_version %d; omitted from print codes",
						t.ID, t.KeyVersion)
					continue
				}
				l.Errorf("look up key version %d: %v", t.KeyVersion, err)
				return nil, status.Error(codes.Internal, "look up signing key")
			}
			secret = key.Secret
			secrets[t.KeyVersion] = secret
		}

		resp.Codes = append(resp.Codes, &v1_catalogpb.TablePrintCode{
			TableId:    t.ID,
			HallId:     t.HallID,
			Label:      t.Label,
			TableCode:  t.TableCode,
			KeyVersion: t.KeyVersion,
			Sig:        qrsig.Sign(secret, in.GetVenueId(), t.TableCode),
		})
	}

	l.Infof("issued %d QR print codes for venue %s to actor %q",
		len(resp.Codes), in.GetVenueId(), in.GetActorStaffId())
	return resp, nil
}

// selectTables resolves the request to the tables to sign: the named ones,
// or every active table when none are named.
func (l *GetTablePrintCodesLogic) selectTables(in *v1_catalogpb.GetTablePrintCodesRequest) ([]model.Table, error) {
	tableModel := model.NewTableModel(l.svcCtx.DB)

	if ids := in.GetTableIds(); len(ids) > 0 {
		out := make([]model.Table, 0, len(ids))
		for _, id := range ids {
			t, err := tableModel.FindByID(l.ctx, in.GetVenueId(), id)
			if err != nil {
				if errors.Is(err, model.ErrNotFound) {
					// Scoped by venue, so an id from another venue lands
					// here — reported as not found rather than confirming
					// that the id exists somewhere.
					return nil, status.Errorf(codes.NotFound, "table %s not found", id)
				}
				l.Errorf("look up table %s: %v", id, err)
				return nil, status.Error(codes.Internal, "look up table")
			}
			out = append(out, *t)
		}
		return out, nil
	}

	var all []model.Table
	var cursor model.Cursor
	for {
		page, err := tableModel.List(l.ctx, in.GetVenueId(), "", cursor, printCodePageSize)
		if err != nil {
			l.Errorf("list tables for venue %s: %v", in.GetVenueId(), err)
			return nil, status.Error(codes.Internal, "list tables")
		}
		for _, t := range page {
			if t.IsActive {
				all = append(all, t)
			}
		}
		if len(page) < printCodePageSize {
			return all, nil
		}
		last := page[len(page)-1]
		cursor = model.Cursor{CreatedAt: last.CreatedAt, ID: last.ID}
	}
}
