package catalogservicelogic

import (
	"context"

	"github.com/menli02/QR-menu/proto/catalog/v1"
	"github.com/menli02/QR-menu/services/catalog/internal/model"
	"github.com/menli02/QR-menu/services/catalog/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type CreateModifierGroupLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCreateModifierGroupLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateModifierGroupLogic {
	return &CreateModifierGroupLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *CreateModifierGroupLogic) CreateModifierGroup(in *v1_catalogpb.CreateModifierGroupRequest) (*v1_catalogpb.ModifierGroup, error) {
	name := in.GetName().GetTranslations()
	if len(name) == 0 {
		return nil, status.Error(codes.InvalidArgument, "name is required")
	}
	if in.GetMinSelect() < 0 || in.GetMaxSelect() < in.GetMinSelect() {
		return nil, status.Error(codes.InvalidArgument, "max_select must be >= min_select >= 0")
	}

	var group *model.ModifierGroup
	var options []model.ModifierOption
	err := l.svcCtx.DB.TransactCtx(l.ctx, func(ctx context.Context, session sqlx.Session) error {
		var err error
		group, err = model.NewModifierGroupModel(session).Insert(ctx, in.GetItemId(), name, in.GetMinSelect(), in.GetMaxSelect(), in.GetRequired())
		if err != nil {
			return err
		}

		optionRows := make([]model.ModifierOption, len(in.GetOptions()))
		for i, o := range in.GetOptions() {
			optionRows[i] = model.ModifierOption{
				Name:            o.GetName().GetTranslations(),
				PriceDeltaMinor: o.GetPriceDelta().GetAmountMinor(),
				SortOrder:       o.GetSortOrder(),
			}
		}
		options, err = model.NewModifierOptionModel(session).ReplaceForGroup(ctx, group.ID, optionRows)
		if err != nil {
			return err
		}

		_, err = model.NewVenueModel(session).BumpMenuVersion(ctx, in.GetVenueId())
		return err
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "create modifier group: %v", err)
	}

	// Currency for the options' Money fields: modifier price deltas are
	// always in the owning venue's currency (A1), same as base_price.
	venue, err := model.NewVenueModel(l.svcCtx.DB).FindByID(l.ctx, in.GetVenueId())
	if err != nil {
		return nil, status.Errorf(codes.Internal, "look up venue: %v", err)
	}
	return modifierGroupToProto(group, options, venue.Currency), nil
}
