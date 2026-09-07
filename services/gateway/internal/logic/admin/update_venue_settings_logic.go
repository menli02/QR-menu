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

type UpdateVenueSettingsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewUpdateVenueSettingsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateVenueSettingsLogic {
	return &UpdateVenueSettingsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// UpdateVenueSettings writes the venue configuration (FR-A2).
//
// The public body has no venueId field and the id is taken from the token,
// which is what stops an admin editing another venue's settings.
//
// Two settings are deliberately not writable through this route even
// though they exist on the underlying message: orderItemCommentMaxLen and
// orderTotalLimitMinor. gateway.api's UpdateVenueSettingsReq omits them,
// and inventing fields the contract doesn't have would leave the frontend
// building against a shape that isn't specified. Sending zeros for them
// instead would be worse still — the order service reads both, and a zero
// limit means "no limit". They keep their stored values because catalog's
// update only writes the fields it is given.
func (l *UpdateVenueSettingsLogic) UpdateVenueSettings(req *types.UpdateVenueSettingsReq) (resp *types.VenueSettings, err error) {
	claims, err := authz.Admin(l.ctx)
	if err != nil {
		return nil, err
	}
	if req.Name == "" || req.Currency == "" || req.DefaultLocale == "" || req.Timezone == "" {
		return nil, errs.New(errs.CodeValidationFailed, "name, currency, defaultLocale and timezone are required")
	}
	if req.ServiceChargeBps < 0 || req.ServiceChargeBps > 10000 {
		return nil, errs.New(errs.CodeValidationFailed, "serviceChargeBps must be between 0 and 10000")
	}

	current, err := l.svcCtx.CatalogRpc.GetVenueSettings(l.ctx, &v1_catalogpb.GetVenueSettingsRequest{
		VenueId: claims.VenueID,
	})
	if err != nil {
		return nil, rpcerr.FromCatalog(err)
	}

	settings := &v1_catalogpb.VenueSettings{
		VenueId:                  claims.VenueID,
		Name:                     req.Name,
		LogoUrl:                  req.LogoUrl,
		Currency:                 req.Currency,
		Locales:                  req.Locales,
		DefaultLocale:            req.DefaultLocale,
		Timezone:                 req.Timezone,
		ServiceChargeBps:         req.ServiceChargeBps,
		CancelWindowSeconds:      req.CancelWindowSeconds,
		KdsAmberThresholdSeconds: req.KdsAmberThresholdSeconds,
		KdsRedThresholdSeconds:   req.KdsRedThresholdSeconds,
		// Carried through from the stored values — see the note above.
		OrderItemCommentMaxLen: current.GetOrderItemCommentMaxLen(),
		OrderTotalLimitMinor:   current.GetOrderTotalLimitMinor(),
	}

	updated, err := l.svcCtx.CatalogRpc.UpdateVenueSettings(l.ctx, &v1_catalogpb.UpdateVenueSettingsRequest{
		Settings:     settings,
		ActorStaffId: claims.StaffID,
	})
	if err != nil {
		return nil, rpcerr.FromCatalog(err)
	}

	out := convert.VenueSettings(updated)
	return &out, nil
}
