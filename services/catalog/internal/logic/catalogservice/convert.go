package catalogservicelogic

import (
	"database/sql"

	"github.com/menli02/QR-menu/proto/catalog/v1"
	"github.com/menli02/QR-menu/services/catalog/internal/model"

	"google.golang.org/protobuf/types/known/timestamppb"
)

func localizedText(m map[string]string) *v1_catalogpb.LocalizedText {
	return &v1_catalogpb.LocalizedText{Translations: m}
}

func money(amountMinor int64, currency string) *v1_catalogpb.Money {
	return &v1_catalogpb.Money{AmountMinor: amountMinor, Currency: currency}
}

func venueSettingsToProto(v *model.Venue) *v1_catalogpb.VenueSettings {
	settings := &v1_catalogpb.VenueSettings{
		VenueId:                  v.ID,
		Name:                     v.Name,
		Currency:                 v.Currency,
		Locales:                  v.Locales,
		DefaultLocale:            v.DefaultLocale,
		Timezone:                 v.Timezone,
		ServiceChargeBps:         v.ServiceChargeBps,
		OrderItemCommentMaxLen:   v.OrderItemCommentMaxLen,
		CancelWindowSeconds:      v.CancelWindowSeconds,
		KdsAmberThresholdSeconds: v.KDSAmberThresholdSeconds,
		KdsRedThresholdSeconds:   v.KDSRedThresholdSeconds,
		UpdatedAt:                timestamppb.New(v.UpdatedAt),
	}
	if v.LogoURL.Valid {
		settings.LogoUrl = v.LogoURL.String
	}
	if v.OrderTotalLimitMinor.Valid {
		settings.OrderTotalLimitMinor = v.OrderTotalLimitMinor.Int64
	}
	return settings
}

func hallToProto(h *model.Hall) *v1_catalogpb.Hall {
	return &v1_catalogpb.Hall{
		Id:        h.ID,
		VenueId:   h.VenueID,
		Name:      h.Name,
		SortOrder: h.SortOrder,
		IsActive:  h.IsActive,
	}
}

func tableToProto(t *model.Table) *v1_catalogpb.Table {
	return &v1_catalogpb.Table{
		Id:         t.ID,
		VenueId:    t.VenueID,
		HallId:     t.HallID,
		Label:      t.Label,
		Seats:      t.Seats,
		IsActive:   t.IsActive,
		TableCode:  t.TableCode,
		KeyVersion: t.KeyVersion,
		CreatedAt:  timestamppb.New(t.CreatedAt),
	}
}

func categoryToProto(c *model.Category) *v1_catalogpb.Category {
	return &v1_catalogpb.Category{
		Id:        c.ID,
		VenueId:   c.VenueID,
		Name:      localizedText(c.Name),
		SortOrder: c.SortOrder,
		IsVisible: c.IsVisible,
		ImageUrl:  c.ImageURL,
	}
}

// currency is threaded through from the owning item/venue (A1: price is
// always in the venue's currency — never stored per-option/per-item).
func modifierOptionToProto(o *model.ModifierOption, currency string) *v1_catalogpb.ModifierOption {
	return &v1_catalogpb.ModifierOption{
		Id:         o.ID,
		GroupId:    o.GroupID,
		Name:       localizedText(o.Name),
		PriceDelta: money(o.PriceDeltaMinor, currency),
		SortOrder:  o.SortOrder,
	}
}

func modifierGroupToProto(g *model.ModifierGroup, options []model.ModifierOption, currency string) *v1_catalogpb.ModifierGroup {
	protoOptions := make([]*v1_catalogpb.ModifierOption, len(options))
	for i := range options {
		protoOptions[i] = modifierOptionToProto(&options[i], currency)
	}
	return &v1_catalogpb.ModifierGroup{
		Id:        g.ID,
		ItemId:    g.ItemID,
		Name:      localizedText(g.Name),
		MinSelect: g.MinSelect,
		MaxSelect: g.MaxSelect,
		Required:  g.Required,
		Options:   protoOptions,
	}
}

func availabilityWindowToProto(w *model.AvailabilityWindow) *v1_catalogpb.AvailabilityWindow {
	return &v1_catalogpb.AvailabilityWindow{
		StartMinuteOfDay: w.StartMinuteOfDay,
		EndMinuteOfDay:   w.EndMinuteOfDay,
	}
}

// itemToProto assembles one wire MenuItem from its row plus its already-
// fetched (not lazily queried — see hydrate.go) modifier groups, their
// options, and its availability windows.
func itemToProto(item *model.MenuItem, currency string, groups []model.ModifierGroup, optionsByGroup map[string][]model.ModifierOption, windows []model.AvailabilityWindow) *v1_catalogpb.MenuItem {
	protoGroups := make([]*v1_catalogpb.ModifierGroup, len(groups))
	for i := range groups {
		protoGroups[i] = modifierGroupToProto(&groups[i], optionsByGroup[groups[i].ID], currency)
	}
	protoWindows := make([]*v1_catalogpb.AvailabilityWindow, len(windows))
	for i := range windows {
		protoWindows[i] = availabilityWindowToProto(&windows[i])
	}
	return &v1_catalogpb.MenuItem{
		Id:                  item.ID,
		VenueId:             item.VenueID,
		CategoryId:          item.CategoryID,
		Name:                localizedText(item.Name),
		Description:         localizedText(item.Description),
		BasePrice:           money(item.BasePriceMinor, currency),
		ImageUrl:            item.ImageURL,
		Allergens:           item.Allergens,
		IsActive:            item.IsActive,
		IsAvailable:         item.IsAvailable,
		SortOrder:           item.SortOrder,
		ModifierGroups:      protoGroups,
		AvailabilityWindows: protoWindows,
	}
}

func nullString(s string) sql.NullString {
	if s == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: s, Valid: true}
}

func nullInt64(v int64) sql.NullInt64 {
	if v <= 0 {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: v, Valid: true}
}
