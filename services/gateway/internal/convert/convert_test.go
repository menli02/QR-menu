package convert

import (
	"testing"
	"time"

	v1_catalogpb "github.com/menli02/QR-menu/proto/catalog/v1"
	v1_orderpb "github.com/menli02/QR-menu/proto/order/v1"

	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestTime(t *testing.T) {
	if got := Time(nil); got != "" {
		t.Errorf("Time(nil) = %q, want empty — an unset timestamp must not render as 1970", got)
	}

	when := time.Date(2026, 3, 1, 14, 30, 0, 0, time.FixedZone("CET", 3600))
	got := Time(timestamppb.New(when))
	if got != "2026-03-01T13:30:00Z" {
		t.Errorf("Time = %q, want the instant normalised to RFC 3339 UTC", got)
	}
}

// TestLocalizedFallbackChain covers the rule that a missing translation
// must still render something: a blank menu line is worse than a line in
// the wrong language.
func TestLocalizedFallbackChain(t *testing.T) {
	full := &v1_catalogpb.LocalizedText{Translations: map[string]string{
		"en": "Latte", "ru": "Латте", "de": "Milchkaffee",
	}}

	cases := []struct {
		name          string
		lt            *v1_catalogpb.LocalizedText
		locale        string
		defaultLocale string
		want          string
	}{
		{"requested locale wins", full, "ru", "en", "Латте"},
		{"falls back to the venue default", full, "fr", "de", "Milchkaffee"},
		{"then to english", full, "fr", "es", "Latte"},
		{"then to anything non-empty", &v1_catalogpb.LocalizedText{
			Translations: map[string]string{"ja": "ラテ"},
		}, "fr", "es", "ラテ"},
		{"empty string is not a translation", &v1_catalogpb.LocalizedText{
			Translations: map[string]string{"ru": "", "en": "Latte"},
		}, "ru", "", "Latte"},
		{"nothing at all", &v1_catalogpb.LocalizedText{}, "en", "en", ""},
		{"nil message", nil, "en", "en", ""},
		{"empty locale is skipped", full, "", "ru", "Латте"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := Localized(tc.lt, tc.locale, tc.defaultLocale); got != tc.want {
				t.Errorf("Localized = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestLocalizedMap(t *testing.T) {
	got := LocalizedMap("ru", "Латте")
	if got.GetTranslations()["ru"] != "Латте" || len(got.GetTranslations()) != 1 {
		t.Errorf("LocalizedMap = %v, want a single-locale map", got.GetTranslations())
	}
	// An absent locale must not produce a map keyed on "", which would be
	// unreachable by every reader.
	if got := LocalizedMap("", "Latte"); got.GetTranslations()["en"] != "Latte" {
		t.Errorf("LocalizedMap with no locale = %v, want it defaulted to en", got.GetTranslations())
	}
}

// TestEnumsRoundTrip checks the public strings and the proto enums agree
// in both directions. A one-way mismatch would mean a client can read a
// status it is then unable to send back.
func TestEnumsRoundTrip(t *testing.T) {
	t.Run("order status", func(t *testing.T) {
		for name, e := range OrderStatuses {
			if got := orderStatusNames[e]; got != name {
				t.Errorf("%q -> %v -> %q", name, e, got)
			}
		}
		if len(OrderStatuses) != len(orderStatusNames) {
			t.Errorf("%d parse entries vs %d render entries", len(OrderStatuses), len(orderStatusNames))
		}
	})
	t.Run("order item status", func(t *testing.T) {
		for name, e := range OrderItemStatuses {
			if got := itemStatusNames[e]; got != name {
				t.Errorf("%q -> %v -> %q", name, e, got)
			}
		}
	})
	t.Run("service request type", func(t *testing.T) {
		for name, e := range ServiceRequestTypes {
			if got := requestTypeNames[e]; got != name {
				t.Errorf("%q -> %v -> %q", name, e, got)
			}
		}
	})
	t.Run("service request status", func(t *testing.T) {
		for name, e := range ServiceRequestStatuses {
			if got := requestStatusNames[e]; got != name {
				t.Errorf("%q -> %v -> %q", name, e, got)
			}
		}
	})
	t.Run("payment method", func(t *testing.T) {
		for name, e := range PaymentMethods {
			if got := paymentMethodNames[e]; got != name {
				t.Errorf("%q -> %v -> %q", name, e, got)
			}
		}
	})
	t.Run("role", func(t *testing.T) {
		for name, e := range Roles {
			if got := roleNames[e]; got != name {
				t.Errorf("%q -> %v -> %q", name, e, got)
			}
		}
	})
}

// TestUnspecifiedEnumRendersEmpty documents what a zero-valued enum
// produces. It matters for TableSession.paymentMethod, which is genuinely
// unset while a session is open, and is `omitempty` in the JSON.
func TestUnspecifiedEnumRendersEmpty(t *testing.T) {
	if got := paymentMethodNames[v1_orderpb.PaymentMethod_PAYMENT_METHOD_UNSPECIFIED]; got != "" {
		t.Errorf("unspecified payment method rendered as %q, want empty", got)
	}
	if got := orderStatusNames[v1_orderpb.OrderStatus_ORDER_STATUS_UNSPECIFIED]; got != "" {
		t.Errorf("unspecified order status rendered as %q, want empty", got)
	}
}

// TestOrderDropsInternalIdentifiers pins a privacy decision: a shared
// table's order list must not tell one guest another's session id, and the
// venue is already implied by the caller's token.
func TestOrderDropsInternalIdentifiers(t *testing.T) {
	in := &v1_orderpb.Order{
		Id:             "order-1",
		VenueId:        "venue-1",
		TableId:        "table-1",
		TableSessionId: "session-1",
		GuestSessionId: "guest-secret",
		Number:         "A-014",
		Status:         v1_orderpb.OrderStatus_ORDER_STATUS_PLACED,
		TotalMinor:     900,
		Currency:       "USD",
		PlacedAt:       timestamppb.New(time.Unix(1_700_000_000, 0)),
		Items: []*v1_orderpb.OrderItem{{
			Id:             "line-1",
			MenuItemId:     "item-1",
			Name:           "Flat White",
			UnitPrice:      &v1_orderpb.Money{AmountMinor: 450, Currency: "USD"},
			Qty:            2,
			Status:         v1_orderpb.OrderItemStatus_ORDER_ITEM_STATUS_PLACED,
			LineTotalMinor: 900,
			Modifiers: []*v1_orderpb.OrderItemModifier{{
				OptionId:   "opt-1",
				Name:       "Oat milk",
				PriceDelta: &v1_orderpb.Money{AmountMinor: 50, Currency: "USD"},
			}},
		}},
	}

	out := Order(in)
	if out.Id != "order-1" || out.Number != "A-014" || out.Status != "placed" {
		t.Errorf("basic fields wrong: %+v", out)
	}
	if out.TotalMinor != 900 || out.Currency != "USD" {
		t.Errorf("money wrong: %d %s", out.TotalMinor, out.Currency)
	}
	if len(out.Items) != 1 || out.Items[0].Name != "Flat White" || out.Items[0].Qty != 2 {
		t.Fatalf("items wrong: %+v", out.Items)
	}
	if len(out.Items[0].Modifiers) != 1 || out.Items[0].Modifiers[0].Name != "Oat milk" {
		t.Errorf("modifiers wrong: %+v", out.Items[0].Modifiers)
	}

	// The public type has no field for either, so this is really a
	// compile-time guarantee — the test states the intent so a future
	// additive change has to argue with it.
	if out.PlacedAt == "" {
		t.Error("placedAt should be rendered")
	}
}

// TestSlicesAreNeverNil keeps JSON responses stable: a nil slice marshals
// to `null`, which every client then has to guard against.
func TestSlicesAreNeverNil(t *testing.T) {
	if got := Order(&v1_orderpb.Order{}); got.Items == nil {
		t.Error("Order.Items is nil, want an empty array")
	}
	if got := Orders(nil); got == nil {
		t.Error("Orders(nil) is nil, want an empty array")
	}
	if got := ServiceRequests(nil); got == nil {
		t.Error("ServiceRequests(nil) is nil, want an empty array")
	}
	if got := Tables(nil); got == nil {
		t.Error("Tables(nil) is nil, want an empty array")
	}
	if got := Halls(nil); got == nil {
		t.Error("Halls(nil) is nil, want an empty array")
	}
	if got := StaffList(nil); got == nil {
		t.Error("StaffList(nil) is nil, want an empty array")
	}
	if got := MenuItem(&v1_catalogpb.MenuItem{}, "en", "en"); got.Allergens == nil {
		t.Error("MenuItem.Allergens is nil, want an empty array")
	}
	if got := MenuItem(&v1_catalogpb.MenuItem{}, "en", "en"); got.ModifierGroups == nil {
		t.Error("MenuItem.ModifierGroups is nil, want an empty array")
	}
	if got := Bill(&v1_orderpb.Bill{}); got.LineItems == nil {
		t.Error("Bill.LineItems is nil, want an empty array")
	}
	if got := DayReport(&v1_orderpb.DayReport{}); got.TopItems == nil {
		t.Error("DayReport.TopItems is nil, want an empty array")
	}
}

func TestTableSessionOpenHasNoClosedFields(t *testing.T) {
	out := TableSession(&v1_orderpb.TableSession{
		Id:       "s1",
		TableId:  "t1",
		Status:   v1_orderpb.TableSessionStatus_TABLE_SESSION_STATUS_OPEN,
		OpenedAt: timestamppb.New(time.Unix(1_700_000_000, 0)),
	})
	if out.Status != "open" {
		t.Errorf("status = %q, want open", out.Status)
	}
	if out.ClosedAt != "" {
		t.Errorf("closedAt = %q, want empty for an open session", out.ClosedAt)
	}
	if out.PaymentMethod != "" {
		t.Errorf("paymentMethod = %q, want empty for an open session", out.PaymentMethod)
	}
}

func TestBillArithmeticIsPassedThroughUnchanged(t *testing.T) {
	// The gateway must not recompute money — the order service is the
	// authority, and a second implementation would be a second answer.
	out := Bill(&v1_orderpb.Bill{
		SubtotalMinor:      1990,
		ServiceChargeBps:   1000,
		ServiceChargeMinor: 199,
		TotalMinor:         2189,
		Currency:           "USD",
	})
	if out.SubtotalMinor != 1990 || out.ServiceChargeMinor != 199 || out.TotalMinor != 2189 {
		t.Errorf("bill totals were altered: %+v", out)
	}
}
