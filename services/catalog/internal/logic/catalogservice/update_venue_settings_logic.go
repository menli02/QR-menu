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

type UpdateVenueSettingsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewUpdateVenueSettingsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateVenueSettingsLogic {
	return &UpdateVenueSettingsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// UpdateVenueSettings never touches menu_version: none of these fields
// (service charge, thresholds, locales, ...) affect what menu content is
// shown, only how orders/tickets around it behave.
func (l *UpdateVenueSettingsLogic) UpdateVenueSettings(in *v1_catalogpb.UpdateVenueSettingsRequest) (*v1_catalogpb.VenueSettings, error) {
	s := in.GetSettings()
	if s == nil {
		return nil, status.Error(codes.InvalidArgument, "settings is required")
	}
	if s.GetName() == "" || s.GetCurrency() == "" {
		return nil, status.Error(codes.InvalidArgument, "name and currency are required")
	}
	if s.GetServiceChargeBps() < 0 || s.GetServiceChargeBps() > 10000 {
		return nil, status.Error(codes.InvalidArgument, "service_charge_bps must be between 0 and 10000")
	}

	venueModel := model.NewVenueModel(l.svcCtx.DB)
	current, err := venueModel.FindByID(l.ctx, s.GetVenueId())
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			return nil, status.Error(codes.NotFound, "venue not found")
		}
		return nil, status.Errorf(codes.Internal, "look up venue: %v", err)
	}

	current.Name = s.GetName()
	current.LogoURL = nullString(s.GetLogoUrl())
	current.Currency = s.GetCurrency()
	current.Locales = s.GetLocales()
	current.DefaultLocale = s.GetDefaultLocale()
	current.Timezone = s.GetTimezone()
	current.ServiceChargeBps = s.GetServiceChargeBps()
	current.OrderItemCommentMaxLen = s.GetOrderItemCommentMaxLen()
	current.OrderTotalLimitMinor = nullInt64(s.GetOrderTotalLimitMinor())
	current.CancelWindowSeconds = s.GetCancelWindowSeconds()
	current.KDSAmberThresholdSeconds = s.GetKdsAmberThresholdSeconds()
	current.KDSRedThresholdSeconds = s.GetKdsRedThresholdSeconds()
	current.BusinessDayCutoffMinute = s.GetBusinessDayCutoffMinute()

	updated, err := venueModel.UpdateSettings(l.ctx, current)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "update venue settings: %v", err)
	}
	return venueSettingsToProto(updated), nil
}
