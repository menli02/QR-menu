package catalogservicelogic

import (
	"context"
	"errors"

	"github.com/menli02/QR-menu/proto/catalog/v1"
	"github.com/menli02/QR-menu/services/catalog/internal/model"
	"github.com/menli02/QR-menu/services/catalog/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type SetItemAvailabilityLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewSetItemAvailabilityLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SetItemAvailabilityLogic {
	return &SetItemAvailabilityLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// itemAvailabilityChangedPayload is the qrmenu.catalog.v1
// catalog.item_availability_changed event body (docs/TZ.md §8.3).
type itemAvailabilityChangedPayload struct {
	ItemID       string `json:"item_id"`
	IsAvailable  bool   `json:"is_available"`
	ActorStaffID string `json:"actor_staff_id,omitempty"`
}

// SetItemAvailability toggles the stop-list state (FR-C4, FR-K6): changes
// must reach guest clients within 5s, which is what the outbox/Kafka
// relay (not yet built — see OutboxModel's comment) exists to drive.
func (l *SetItemAvailabilityLogic) SetItemAvailability(in *v1_catalogpb.SetItemAvailabilityRequest) (*v1_catalogpb.SetItemAvailabilityResponse, error) {
	venue, err := model.NewVenueModel(l.svcCtx.DB).FindByID(l.ctx, in.GetVenueId())
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			return nil, status.Error(codes.NotFound, "venue not found")
		}
		return nil, status.Errorf(codes.Internal, "look up venue: %v", err)
	}

	var updated *model.MenuItem
	err = l.svcCtx.DB.TransactCtx(l.ctx, func(ctx context.Context, session sqlx.Session) error {
		var err error
		updated, err = model.NewMenuItemModel(session).SetAvailability(ctx, in.GetItemId(), in.GetVenueId(), in.GetIsAvailable())
		if err != nil {
			return err
		}
		if _, err := model.NewVenueModel(session).BumpMenuVersion(ctx, in.GetVenueId()); err != nil {
			return err
		}
		return model.NewOutboxModel(session).Insert(ctx, "catalog.item_availability_changed", in.GetVenueId(),
			"qrmenu.catalog.v1", in.GetItemId(),
			itemAvailabilityChangedPayload{ItemID: in.GetItemId(), IsAvailable: in.GetIsAvailable(), ActorStaffID: in.GetActorStaffId()},
			"")
	})
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			return nil, status.Error(codes.NotFound, "menu item not found")
		}
		return nil, status.Errorf(codes.Internal, "set item availability: %v", err)
	}

	item, err := hydrateItem(l.ctx, l.svcCtx.DB, updated, venue.Currency)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "load modifiers: %v", err)
	}
	return &v1_catalogpb.SetItemAvailabilityResponse{Item: item}, nil
}
