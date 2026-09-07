// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package admin

import (
	"context"

	v1_catalogpb "github.com/menli02/QR-menu/proto/catalog/v1"
	"github.com/menli02/QR-menu/services/gateway/internal/authz"
	"github.com/menli02/QR-menu/services/gateway/internal/convert"
	"github.com/menli02/QR-menu/services/gateway/internal/errs"
	"github.com/menli02/QR-menu/services/gateway/internal/rpcerr"
	"github.com/menli02/QR-menu/services/gateway/internal/svc"
	"github.com/menli02/QR-menu/services/gateway/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type CreateTableLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewCreateTableLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateTableLogic {
	return &CreateTableLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// CreateTable adds a table and generates its signed QR code (FR-T1, FR-T2).
//
// Known gap: the response carries tableCode and keyVersion but not the
// HMAC signature, because neither the catalog Table message nor the public
// Table type has a field for it — and without the signature the printed QR
// link cannot be assembled. The same gap blocks GET /admin/tables/qr.pdf.
// Closing it needs an additive `sig` field or a dedicated "get printable
// code" RPC; it is flagged here rather than worked around, because the
// alternative is the gateway re-deriving an HMAC whose key lives in
// catalog.
func (l *CreateTableLogic) CreateTable(req *types.CreateTableReq) (resp *types.Table, err error) {
	claims, err := authz.Admin(l.ctx)
	if err != nil {
		return nil, err
	}
	if req.HallId == "" || req.Label == "" {
		return nil, errs.New(errs.CodeValidationFailed, "hallId and label are required")
	}
	if req.Seats < 0 {
		return nil, errs.New(errs.CodeValidationFailed, "seats cannot be negative")
	}

	table, err := l.svcCtx.CatalogRpc.CreateTable(l.ctx, &v1_catalogpb.CreateTableRequest{
		VenueId: claims.VenueID,
		HallId:  req.HallId,
		Label:   req.Label,
		Seats:   req.Seats,
	})
	if err != nil {
		return nil, rpcerr.FromCatalog(err)
	}

	out := convert.Table(table)
	return &out, nil
}
