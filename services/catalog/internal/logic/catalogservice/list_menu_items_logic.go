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

const (
	defaultMenuItemPageSize = 50
	maxMenuItemPageSize     = 200
)

type ListMenuItemsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListMenuItemsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListMenuItemsLogic {
	return &ListMenuItemsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ListMenuItemsLogic) ListMenuItems(in *v1_catalogpb.ListMenuItemsRequest) (*v1_catalogpb.ListMenuItemsResponse, error) {
	venue, err := model.NewVenueModel(l.svcCtx.DB).FindByID(l.ctx, in.GetVenueId())
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			return nil, status.Error(codes.NotFound, "venue not found")
		}
		return nil, status.Errorf(codes.Internal, "look up venue: %v", err)
	}

	cursor, err := model.DecodeCursor(in.GetCursor())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid cursor: %v", err)
	}
	limit := int(in.GetPageSize())
	if limit <= 0 || limit > maxMenuItemPageSize {
		limit = defaultMenuItemPageSize
	}

	rows, err := model.NewMenuItemModel(l.svcCtx.DB).ListByVenue(l.ctx, in.GetVenueId(), in.GetCategoryId(), cursor, limit)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list menu items: %v", err)
	}

	items, err := hydrateItems(l.ctx, l.svcCtx.DB, rows, venue.Currency)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "load modifiers: %v", err)
	}

	resp := &v1_catalogpb.ListMenuItemsResponse{Items: items}
	if len(rows) == limit {
		last := rows[len(rows)-1]
		resp.NextCursor = model.Cursor{CreatedAt: last.CreatedAt, ID: last.ID}.Encode()
	}
	return resp, nil
}
