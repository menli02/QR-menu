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

type UpdateModifierGroupLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewUpdateModifierGroupLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateModifierGroupLogic {
	return &UpdateModifierGroupLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// UpdateModifierGroup replaces the group's whole option set wholesale
// (proto has no "add one option" RPC — see ModifierOptionModel.ReplaceForGroup).
func (l *UpdateModifierGroupLogic) UpdateModifierGroup(in *v1_catalogpb.UpdateModifierGroupRequest) (*v1_catalogpb.ModifierGroup, error) {
	g := in.GetGroup()
	if g == nil || len(g.GetName().GetTranslations()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "group with a name is required")
	}
	if g.GetMinSelect() < 0 || g.GetMaxSelect() < g.GetMinSelect() {
		return nil, status.Error(codes.InvalidArgument, "max_select must be >= min_select >= 0")
	}

	var updated *model.ModifierGroup
	var options []model.ModifierOption
	err := l.svcCtx.DB.TransactCtx(l.ctx, func(ctx context.Context, session sqlx.Session) error {
		var err error
		updated, err = model.NewModifierGroupModel(session).Update(ctx, g.GetId(), g.GetItemId(),
			g.GetName().GetTranslations(), g.GetMinSelect(), g.GetMaxSelect(), g.GetRequired())
		if err != nil {
			return err
		}

		optionRows := make([]model.ModifierOption, len(g.GetOptions()))
		for i, o := range g.GetOptions() {
			optionRows[i] = model.ModifierOption{
				Name:            o.GetName().GetTranslations(),
				PriceDeltaMinor: o.GetPriceDelta().GetAmountMinor(),
				SortOrder:       o.GetSortOrder(),
			}
		}
		options, err = model.NewModifierOptionModel(session).ReplaceForGroup(ctx, updated.ID, optionRows)
		if err != nil {
			return err
		}

		_, err = model.NewVenueModel(session).BumpMenuVersion(ctx, in.GetVenueId())
		return err
	})
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			return nil, status.Error(codes.NotFound, "modifier group not found")
		}
		return nil, status.Errorf(codes.Internal, "update modifier group: %v", err)
	}

	venue, err := model.NewVenueModel(l.svcCtx.DB).FindByID(l.ctx, in.GetVenueId())
	if err != nil {
		return nil, status.Errorf(codes.Internal, "look up venue: %v", err)
	}
	return modifierGroupToProto(updated, options, venue.Currency), nil
}
