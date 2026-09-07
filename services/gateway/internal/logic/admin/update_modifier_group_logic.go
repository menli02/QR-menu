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

type UpdateModifierGroupLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewUpdateModifierGroupLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateModifierGroupLogic {
	return &UpdateModifierGroupLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// UpdateModifierGroup replaces the group's whole option set.
//
// Unlike the group's own name, option names are *not* merged across
// locales: the option list is sent complete and catalog rewrites it, so
// options carry only the locale in this request. That falls out of the
// contract having no per-option update — there is no stored row to merge
// against once the set is replaced. Worth knowing before building a
// multi-locale modifier editor.
func (l *UpdateModifierGroupLogic) UpdateModifierGroup(req *types.UpdateModifierGroupReq) (resp *types.ModifierGroup, err error) {
	claims, err := authz.Admin(l.ctx)
	if err != nil {
		return nil, err
	}
	if req.GroupId == "" {
		return nil, errs.New(errs.CodeValidationFailed, "groupId is required")
	}
	if err := validateModifierGroup(req.ItemId, req.Name, req.MinSelect, req.MaxSelect, req.Required, len(req.Options)); err != nil {
		return nil, err
	}

	group, err := l.svcCtx.CatalogRpc.UpdateModifierGroup(l.ctx, &v1_catalogpb.UpdateModifierGroupRequest{
		VenueId: claims.VenueID,
		Group: &v1_catalogpb.ModifierGroup{
			Id:        req.GroupId,
			ItemId:    req.ItemId,
			Name:      convert.LocalizedMap(req.Locale, req.Name),
			MinSelect: req.MinSelect,
			MaxSelect: req.MaxSelect,
			Required:  req.Required,
			Options:   modifierOptions(req.Locale, req.Options),
		},
	})
	if err != nil {
		return nil, rpcerr.FromCatalog(err)
	}

	out := convert.ModifierGroup(group, req.Locale, req.Locale)
	return &out, nil
}

// validateModifierGroup checks the selection bounds are coherent before
// they reach catalog. These are the rules ResolveOrderItems enforces at
// order time (FR-C3), so a group that violates them would make its item
// permanently unorderable — much better caught on the admin form.
func validateModifierGroup(itemID, name string, minSelect, maxSelect int32, required bool, optionCount int) error {
	switch {
	case itemID == "":
		return errs.New(errs.CodeValidationFailed, "itemId is required")
	case name == "":
		return errs.New(errs.CodeValidationFailed, "name is required")
	case minSelect < 0:
		return errs.New(errs.CodeValidationFailed, "minSelect cannot be negative")
	case maxSelect < 1:
		return errs.New(errs.CodeValidationFailed, "maxSelect must be at least 1")
	case minSelect > maxSelect:
		return errs.New(errs.CodeValidationFailed, "minSelect cannot exceed maxSelect")
	case int(maxSelect) > optionCount:
		return errs.New(errs.CodeValidationFailed, "maxSelect cannot exceed the number of options")
	case required && minSelect == 0:
		// A required group with minSelect 0 is contradictory, and
		// ResolveOrderItems reads it as "at least one" anyway. Rejecting
		// it keeps the stored rule and the enforced rule the same.
		return errs.New(errs.CodeValidationFailed, "a required group must have minSelect of at least 1")
	}
	return nil
}

func modifierOptions(locale string, in []types.ModifierOption) []*v1_catalogpb.ModifierOption {
	out := make([]*v1_catalogpb.ModifierOption, 0, len(in))
	for i, o := range in {
		out = append(out, &v1_catalogpb.ModifierOption{
			Id:   o.Id,
			Name: convert.LocalizedMap(locale, o.Name),
			// Currency comes from the venue (A1); a per-option one would
			// be a second source of truth.
			PriceDelta: &v1_catalogpb.Money{AmountMinor: o.PriceDelta.AmountMinor},
			SortOrder:  int32(i),
		})
	}
	return out
}
