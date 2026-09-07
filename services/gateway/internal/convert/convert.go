// Package convert maps backing-service protobuf messages onto the
// gateway's public JSON types.
//
// It is one package rather than one per route group because the same
// public type is produced by several groups — an Order is returned by
// guest, kds and floor endpoints alike — and three copies of that mapping
// would drift.
//
// Two conventions the whole package follows, both of them contract
// decisions rather than style:
//
//   - Enums cross the boundary as the lower-case strings §8.1 documents
//     ("placed", "call_waiter"), never as the proto's SCREAMING_CASE. The
//     public API and the internal proto are separately versioned, and
//     leaking the proto spelling would couple them.
//   - Timestamps are RFC 3339 UTC strings (§8.1), and an unset timestamp
//     becomes "" rather than the zero instant, so a client never renders
//     1970 for "not closed yet".
package convert

import (
	"time"

	v1_catalogpb "github.com/menli02/QR-menu/proto/catalog/v1"
	v1_identitypb "github.com/menli02/QR-menu/proto/identity/v1"
	v1_orderpb "github.com/menli02/QR-menu/proto/order/v1"
	"github.com/menli02/QR-menu/services/gateway/internal/types"

	"google.golang.org/protobuf/types/known/timestamppb"
)

// ---------------------------------------------------------------------
// Primitives
// ---------------------------------------------------------------------

// Time renders a proto timestamp as RFC 3339 UTC, or "" when unset.
func Time(ts *timestamppb.Timestamp) string {
	if ts == nil || !ts.IsValid() {
		return ""
	}
	return ts.AsTime().UTC().Format(time.RFC3339)
}

func catalogMoney(m *v1_catalogpb.Money) types.Money {
	return types.Money{AmountMinor: m.GetAmountMinor(), Currency: m.GetCurrency()}
}

func orderMoney(m *v1_orderpb.Money) types.Money {
	return types.Money{AmountMinor: m.GetAmountMinor(), Currency: m.GetCurrency()}
}

// Localized picks one locale out of a LocalizedText map, falling back the
// same way catalog's own resolver does: requested, then the venue default,
// then English, then any non-empty translation. A menu with a missing
// translation should render *something* rather than a blank line.
func Localized(lt *v1_catalogpb.LocalizedText, locale, defaultLocale string) string {
	m := lt.GetTranslations()
	for _, candidate := range []string{locale, defaultLocale, "en"} {
		if candidate == "" {
			continue
		}
		if v, ok := m[candidate]; ok && v != "" {
			return v
		}
	}
	for _, v := range m {
		if v != "" {
			return v
		}
	}
	return ""
}

// LocalizedMap builds the single-locale write payload the admin API uses:
// §8.1's admin write bodies edit one locale at a time, while the catalog
// aggregate stores the full per-locale map.
//
// Callers that are *updating* must merge into the existing map rather than
// passing this straight through, or editing the English name would delete
// every other translation. See admin.mergeLocalized.
func LocalizedMap(locale, text string) *v1_catalogpb.LocalizedText {
	if locale == "" {
		locale = "en"
	}
	return &v1_catalogpb.LocalizedText{Translations: map[string]string{locale: text}}
}

// ---------------------------------------------------------------------
// Enums — public strings per docs/TZ.md §8.1
// ---------------------------------------------------------------------

var orderStatusNames = map[v1_orderpb.OrderStatus]string{
	v1_orderpb.OrderStatus_ORDER_STATUS_PLACED:      "placed",
	v1_orderpb.OrderStatus_ORDER_STATUS_ACCEPTED:    "accepted",
	v1_orderpb.OrderStatus_ORDER_STATUS_IN_PROGRESS: "in_progress",
	v1_orderpb.OrderStatus_ORDER_STATUS_READY:       "ready",
	v1_orderpb.OrderStatus_ORDER_STATUS_SERVED:      "served",
	v1_orderpb.OrderStatus_ORDER_STATUS_CANCELLED:   "cancelled",
}

// OrderStatuses is the reverse map, used to parse a client's `to` field.
var OrderStatuses = map[string]v1_orderpb.OrderStatus{
	"placed":      v1_orderpb.OrderStatus_ORDER_STATUS_PLACED,
	"accepted":    v1_orderpb.OrderStatus_ORDER_STATUS_ACCEPTED,
	"in_progress": v1_orderpb.OrderStatus_ORDER_STATUS_IN_PROGRESS,
	"ready":       v1_orderpb.OrderStatus_ORDER_STATUS_READY,
	"served":      v1_orderpb.OrderStatus_ORDER_STATUS_SERVED,
	"cancelled":   v1_orderpb.OrderStatus_ORDER_STATUS_CANCELLED,
}

var itemStatusNames = map[v1_orderpb.OrderItemStatus]string{
	v1_orderpb.OrderItemStatus_ORDER_ITEM_STATUS_PLACED:    "placed",
	v1_orderpb.OrderItemStatus_ORDER_ITEM_STATUS_COOKING:   "cooking",
	v1_orderpb.OrderItemStatus_ORDER_ITEM_STATUS_READY:     "ready",
	v1_orderpb.OrderItemStatus_ORDER_ITEM_STATUS_CANCELLED: "cancelled",
}

var OrderItemStatuses = map[string]v1_orderpb.OrderItemStatus{
	"placed":    v1_orderpb.OrderItemStatus_ORDER_ITEM_STATUS_PLACED,
	"cooking":   v1_orderpb.OrderItemStatus_ORDER_ITEM_STATUS_COOKING,
	"ready":     v1_orderpb.OrderItemStatus_ORDER_ITEM_STATUS_READY,
	"cancelled": v1_orderpb.OrderItemStatus_ORDER_ITEM_STATUS_CANCELLED,
}

var sessionStatusNames = map[v1_orderpb.TableSessionStatus]string{
	v1_orderpb.TableSessionStatus_TABLE_SESSION_STATUS_OPEN:   "open",
	v1_orderpb.TableSessionStatus_TABLE_SESSION_STATUS_CLOSED: "closed",
}

var requestTypeNames = map[v1_orderpb.ServiceRequestType]string{
	v1_orderpb.ServiceRequestType_SERVICE_REQUEST_TYPE_CALL_WAITER:  "call_waiter",
	v1_orderpb.ServiceRequestType_SERVICE_REQUEST_TYPE_REQUEST_BILL: "request_bill",
}

var ServiceRequestTypes = map[string]v1_orderpb.ServiceRequestType{
	"call_waiter":  v1_orderpb.ServiceRequestType_SERVICE_REQUEST_TYPE_CALL_WAITER,
	"request_bill": v1_orderpb.ServiceRequestType_SERVICE_REQUEST_TYPE_REQUEST_BILL,
}

var requestStatusNames = map[v1_orderpb.ServiceRequestStatus]string{
	v1_orderpb.ServiceRequestStatus_SERVICE_REQUEST_STATUS_OPEN:         "open",
	v1_orderpb.ServiceRequestStatus_SERVICE_REQUEST_STATUS_ACKNOWLEDGED: "acknowledged",
	v1_orderpb.ServiceRequestStatus_SERVICE_REQUEST_STATUS_RESOLVED:     "resolved",
	v1_orderpb.ServiceRequestStatus_SERVICE_REQUEST_STATUS_EXPIRED:      "expired",
}

var ServiceRequestStatuses = map[string]v1_orderpb.ServiceRequestStatus{
	"open":         v1_orderpb.ServiceRequestStatus_SERVICE_REQUEST_STATUS_OPEN,
	"acknowledged": v1_orderpb.ServiceRequestStatus_SERVICE_REQUEST_STATUS_ACKNOWLEDGED,
	"resolved":     v1_orderpb.ServiceRequestStatus_SERVICE_REQUEST_STATUS_RESOLVED,
	"expired":      v1_orderpb.ServiceRequestStatus_SERVICE_REQUEST_STATUS_EXPIRED,
}

var paymentMethodNames = map[v1_orderpb.PaymentMethod]string{
	v1_orderpb.PaymentMethod_PAYMENT_METHOD_CASH:          "cash",
	v1_orderpb.PaymentMethod_PAYMENT_METHOD_CARD_TERMINAL: "card_terminal",
	v1_orderpb.PaymentMethod_PAYMENT_METHOD_OTHER:         "other",
}

var PaymentMethods = map[string]v1_orderpb.PaymentMethod{
	"cash":          v1_orderpb.PaymentMethod_PAYMENT_METHOD_CASH,
	"card_terminal": v1_orderpb.PaymentMethod_PAYMENT_METHOD_CARD_TERMINAL,
	"other":         v1_orderpb.PaymentMethod_PAYMENT_METHOD_OTHER,
}

var roleNames = map[v1_identitypb.Role]string{
	v1_identitypb.Role_ROLE_ADMIN:   "admin",
	v1_identitypb.Role_ROLE_MANAGER: "manager",
	v1_identitypb.Role_ROLE_WAITER:  "waiter",
	v1_identitypb.Role_ROLE_COOK:    "cook",
}

var Roles = map[string]v1_identitypb.Role{
	"admin":   v1_identitypb.Role_ROLE_ADMIN,
	"manager": v1_identitypb.Role_ROLE_MANAGER,
	"waiter":  v1_identitypb.Role_ROLE_WAITER,
	"cook":    v1_identitypb.Role_ROLE_COOK,
}

// ---------------------------------------------------------------------
// Catalog
// ---------------------------------------------------------------------

func Hall(h *v1_catalogpb.Hall) types.Hall {
	return types.Hall{
		Id:        h.GetId(),
		Name:      h.GetName(),
		SortOrder: h.GetSortOrder(),
		IsActive:  h.GetIsActive(),
	}
}

func Halls(in []*v1_catalogpb.Hall) []types.Hall {
	out := make([]types.Hall, 0, len(in))
	for _, h := range in {
		out = append(out, Hall(h))
	}
	return out
}

func Table(t *v1_catalogpb.Table) types.Table {
	return types.Table{
		Id:         t.GetId(),
		HallId:     t.GetHallId(),
		Label:      t.GetLabel(),
		Seats:      t.GetSeats(),
		IsActive:   t.GetIsActive(),
		TableCode:  t.GetTableCode(),
		KeyVersion: t.GetKeyVersion(),
	}
}

func Tables(in []*v1_catalogpb.Table) []types.Table {
	out := make([]types.Table, 0, len(in))
	for _, t := range in {
		out = append(out, Table(t))
	}
	return out
}

func Category(c *v1_catalogpb.Category, locale, defaultLocale string) types.Category {
	return types.Category{
		Id:        c.GetId(),
		Name:      Localized(c.GetName(), locale, defaultLocale),
		SortOrder: c.GetSortOrder(),
		IsVisible: c.GetIsVisible(),
		ImageUrl:  c.GetImageUrl(),
	}
}

func Categories(in []*v1_catalogpb.Category, locale, defaultLocale string) []types.Category {
	out := make([]types.Category, 0, len(in))
	for _, c := range in {
		out = append(out, Category(c, locale, defaultLocale))
	}
	return out
}

func ModifierOption(o *v1_catalogpb.ModifierOption, locale, defaultLocale string) types.ModifierOption {
	return types.ModifierOption{
		Id:         o.GetId(),
		Name:       Localized(o.GetName(), locale, defaultLocale),
		PriceDelta: catalogMoney(o.GetPriceDelta()),
	}
}

func ModifierGroup(g *v1_catalogpb.ModifierGroup, locale, defaultLocale string) types.ModifierGroup {
	out := types.ModifierGroup{
		Id:        g.GetId(),
		Name:      Localized(g.GetName(), locale, defaultLocale),
		MinSelect: g.GetMinSelect(),
		MaxSelect: g.GetMaxSelect(),
		Required:  g.GetRequired(),
		Options:   make([]types.ModifierOption, 0, len(g.GetOptions())),
	}
	for _, o := range g.GetOptions() {
		out.Options = append(out.Options, ModifierOption(o, locale, defaultLocale))
	}
	return out
}

func MenuItem(i *v1_catalogpb.MenuItem, locale, defaultLocale string) types.MenuItem {
	out := types.MenuItem{
		Id:             i.GetId(),
		CategoryId:     i.GetCategoryId(),
		Name:           Localized(i.GetName(), locale, defaultLocale),
		Description:    Localized(i.GetDescription(), locale, defaultLocale),
		BasePrice:      catalogMoney(i.GetBasePrice()),
		ImageUrl:       i.GetImageUrl(),
		Allergens:      i.GetAllergens(),
		IsAvailable:    i.GetIsAvailable(),
		SortOrder:      i.GetSortOrder(),
		ModifierGroups: make([]types.ModifierGroup, 0, len(i.GetModifierGroups())),
	}
	if out.Allergens == nil {
		out.Allergens = []string{}
	}
	for _, g := range i.GetModifierGroups() {
		out.ModifierGroups = append(out.ModifierGroups, ModifierGroup(g, locale, defaultLocale))
	}
	return out
}

func MenuItems(in []*v1_catalogpb.MenuItem, locale, defaultLocale string) []types.MenuItem {
	out := make([]types.MenuItem, 0, len(in))
	for _, i := range in {
		out = append(out, MenuItem(i, locale, defaultLocale))
	}
	return out
}

func VenueSettings(s *v1_catalogpb.VenueSettings) types.VenueSettings {
	return types.VenueSettings{
		VenueId:                  s.GetVenueId(),
		Name:                     s.GetName(),
		LogoUrl:                  s.GetLogoUrl(),
		Currency:                 s.GetCurrency(),
		Locales:                  s.GetLocales(),
		DefaultLocale:            s.GetDefaultLocale(),
		Timezone:                 s.GetTimezone(),
		ServiceChargeBps:         s.GetServiceChargeBps(),
		CancelWindowSeconds:      s.GetCancelWindowSeconds(),
		KdsAmberThresholdSeconds: s.GetKdsAmberThresholdSeconds(),
		KdsRedThresholdSeconds:   s.GetKdsRedThresholdSeconds(),
	}
}

// ---------------------------------------------------------------------
// Order
// ---------------------------------------------------------------------

func OrderItem(it *v1_orderpb.OrderItem) types.OrderItem {
	out := types.OrderItem{
		Id:             it.GetId(),
		MenuItemId:     it.GetMenuItemId(),
		Name:           it.GetName(),
		UnitPrice:      orderMoney(it.GetUnitPrice()),
		Qty:            it.GetQty(),
		Comment:        it.GetComment(),
		Status:         itemStatusNames[it.GetStatus()],
		LineTotalMinor: it.GetLineTotalMinor(),
		Modifiers:      make([]types.OrderItemModifier, 0, len(it.GetModifiers())),
	}
	for _, m := range it.GetModifiers() {
		out.Modifiers = append(out.Modifiers, types.OrderItemModifier{
			OptionId:   m.GetOptionId(),
			Name:       m.GetName(),
			PriceDelta: orderMoney(m.GetPriceDelta()),
		})
	}
	return out
}

// Order deliberately drops venue_id and guest_session_id from the public
// shape: the venue is implied by the caller's token, and one guest has no
// business learning another's session id from a shared table's order list.
func Order(o *v1_orderpb.Order) types.Order {
	out := types.Order{
		Id:             o.GetId(),
		TableId:        o.GetTableId(),
		TableSessionId: o.GetTableSessionId(),
		Number:         o.GetNumber(),
		Status:         orderStatusNames[o.GetStatus()],
		TotalMinor:     o.GetTotalMinor(),
		Currency:       o.GetCurrency(),
		PlacedAt:       Time(o.GetPlacedAt()),
		UpdatedAt:      Time(o.GetUpdatedAt()),
		Items:          make([]types.OrderItem, 0, len(o.GetItems())),
	}
	for _, it := range o.GetItems() {
		out.Items = append(out.Items, OrderItem(it))
	}
	return out
}

func Orders(in []*v1_orderpb.Order) []types.Order {
	out := make([]types.Order, 0, len(in))
	for _, o := range in {
		out = append(out, Order(o))
	}
	return out
}

func TableSession(s *v1_orderpb.TableSession) types.TableSession {
	return types.TableSession{
		Id:            s.GetId(),
		TableId:       s.GetTableId(),
		Status:        sessionStatusNames[s.GetStatus()],
		OpenedAt:      Time(s.GetOpenedAt()),
		ClosedAt:      Time(s.GetClosedAt()),
		PaymentMethod: paymentMethodNames[s.GetPaymentMethod()],
		TotalMinor:    s.GetTotalMinor(),
		Currency:      s.GetCurrency(),
	}
}

func ServiceRequest(r *v1_orderpb.ServiceRequest) types.ServiceRequest {
	return types.ServiceRequest{
		Id:             r.GetId(),
		TableId:        r.GetTableId(),
		TableSessionId: r.GetTableSessionId(),
		Type:           requestTypeNames[r.GetType()],
		Status:         requestStatusNames[r.GetStatus()],
		Note:           r.GetNote(),
		CreatedAt:      Time(r.GetCreatedAt()),
	}
}

func ServiceRequests(in []*v1_orderpb.ServiceRequest) []types.ServiceRequest {
	out := make([]types.ServiceRequest, 0, len(in))
	for _, r := range in {
		out = append(out, ServiceRequest(r))
	}
	return out
}

func Bill(b *v1_orderpb.Bill) types.Bill {
	out := types.Bill{
		TableSessionId:     b.GetTableSessionId(),
		SubtotalMinor:      b.GetSubtotalMinor(),
		ServiceChargeBps:   b.GetServiceChargeBps(),
		ServiceChargeMinor: b.GetServiceChargeMinor(),
		TotalMinor:         b.GetTotalMinor(),
		Currency:           b.GetCurrency(),
		LineItems:          make([]types.BillLineItem, 0, len(b.GetLineItems())),
	}
	for _, li := range b.GetLineItems() {
		out.LineItems = append(out.LineItems, types.BillLineItem{
			Name:           li.GetName(),
			Qty:            li.GetQty(),
			UnitPrice:      orderMoney(li.GetUnitPrice()),
			LineTotalMinor: li.GetLineTotalMinor(),
		})
	}
	return out
}

func DayReport(r *v1_orderpb.DayReport) types.DayReport {
	out := types.DayReport{
		BusinessDate:         r.GetBusinessDate(),
		OrdersCount:          r.GetOrdersCount(),
		RevenueMinor:         r.GetRevenueMinor(),
		Currency:             r.GetCurrency(),
		AverageTicketMinor:   r.GetAverageTicketMinor(),
		AverageAcceptSeconds: r.GetAverageAcceptSeconds(),
		AverageCookSeconds:   r.GetAverageCookSeconds(),
		TopItems:             make([]types.TopItem, 0, len(r.GetTopItems())),
	}
	for _, t := range r.GetTopItems() {
		out.TopItems = append(out.TopItems, types.TopItem{
			Name:         t.GetName(),
			QtySold:      t.GetQtySold(),
			RevenueMinor: t.GetRevenueMinor(),
		})
	}
	return out
}

// ---------------------------------------------------------------------
// Identity
// ---------------------------------------------------------------------

func Staff(s *v1_identitypb.Staff) types.Staff {
	return types.Staff{
		Id:       s.GetId(),
		Name:     s.GetName(),
		Email:    s.GetEmail(),
		Role:     roleNames[s.GetRole()],
		IsActive: s.GetIsActive(),
	}
}

func StaffList(in []*v1_identitypb.Staff) []types.Staff {
	out := make([]types.Staff, 0, len(in))
	for _, s := range in {
		out = append(out, Staff(s))
	}
	return out
}

func TokenPair(p *v1_identitypb.TokenPair) types.TokenResp {
	return types.TokenResp{
		AccessToken:           p.GetAccessToken(),
		RefreshToken:          p.GetRefreshToken(),
		TokenType:             "Bearer",
		AccessTokenExpiresAt:  Time(p.GetAccessTokenExpiresAt()),
		RefreshTokenExpiresAt: Time(p.GetRefreshTokenExpiresAt()),
	}
}
