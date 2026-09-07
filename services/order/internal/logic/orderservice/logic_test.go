package orderservicelogic

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/menli02/QR-menu/proto/order/v1"
	"github.com/menli02/QR-menu/services/order/internal/venue"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// codeOf pulls the gRPC code out of an apierr, so assertions read as
// "this was rejected as invalid" rather than string-matching a message.
func codeOf(err error) codes.Code {
	if err == nil {
		return codes.OK
	}
	st, _ := status.FromError(err)
	return st.Code()
}

func TestFormatOrderNumber(t *testing.T) {
	cases := []struct {
		seq  int
		want string
	}{
		{1, "A-001"},
		{14, "A-014"}, // the example in FR-O8
		{999, "A-999"},
		{1000, "B-001"},
		{1998, "B-999"},
		{1999, "C-001"},
		{25 * 999, "Y-999"},
		{25*999 + 1, "Z-001"},
		{26 * 999, "Z-999"},
		// Past Z the letter scheme is exhausted; a plain sequence is
		// wrong-looking but unique, which beats colliding with the
		// morning's A-001.
		{26*999 + 1, "25975"},
		// Defensive: a counter that somehow came back below 1.
		{0, "A-001"},
		{-5, "A-001"},
	}
	for _, tc := range cases {
		if got := formatOrderNumber(tc.seq); got != tc.want {
			t.Errorf("formatOrderNumber(%d) = %q, want %q", tc.seq, got, tc.want)
		}
	}
}

func TestFormatOrderNumberIsUniquePerDay(t *testing.T) {
	seen := make(map[string]int, 2000)
	for seq := 1; seq <= 2000; seq++ {
		n := formatOrderNumber(seq)
		if prev, dup := seen[n]; dup {
			t.Fatalf("formatOrderNumber produced %q for both seq %d and %d", n, prev, seq)
		}
		seen[n] = seq
	}
}

func TestServiceCharge(t *testing.T) {
	cases := []struct {
		name     string
		subtotal int64
		bps      int32
		want     int64
	}{
		{"no charge configured", 10000, 0, 0},
		{"negative bps is ignored", 10000, -100, 0},
		{"zero subtotal", 0, 1000, 0},
		{"ten percent of a round number", 10000, 1000, 1000},
		{"twelve and a half percent", 10000, 1250, 1250},
		// The rounding case the comment on serviceCharge calls out: plain
		// integer division would give 123 and under-bill by one unit.
		{"rounds half up rather than truncating", 1234, 1000, 123},
		{"rounds up past the halfway point", 1235, 1000, 124},
		{"exactly half rounds up", 5, 1000, 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := serviceCharge(tc.subtotal, tc.bps); got != tc.want {
				t.Errorf("serviceCharge(%d, %d) = %d, want %d", tc.subtotal, tc.bps, got, tc.want)
			}
		})
	}
}

func TestAverageTicket(t *testing.T) {
	cases := []struct {
		revenue int64
		orders  int32
		want    int64
	}{
		{0, 0, 0},
		{1000, 0, 0}, // no divide by zero on an empty day
		{1000, 4, 250},
		{1000, 3, 333},
		{1001, 3, 334}, // 333.67 rounds up
		{1000, 1, 1000},
	}
	for _, tc := range cases {
		if got := averageTicket(tc.revenue, tc.orders); got != tc.want {
			t.Errorf("averageTicket(%d, %d) = %d, want %d", tc.revenue, tc.orders, got, tc.want)
		}
	}
}

func TestValidateCreateOrder(t *testing.T) {
	valid := func() *v1_orderpb.CreateOrderRequest {
		return &v1_orderpb.CreateOrderRequest{
			VenueId:        "venue",
			TableId:        "table",
			GuestSessionId: "guest",
			Items:          []*v1_orderpb.OrderItemRequest{{ItemId: "item", Qty: 1}},
		}
	}

	if err := validateCreateOrder(valid()); err != nil {
		t.Fatalf("a valid request was rejected: %v", err)
	}

	cases := []struct {
		name   string
		mutate func(*v1_orderpb.CreateOrderRequest)
	}{
		{"missing venue", func(r *v1_orderpb.CreateOrderRequest) { r.VenueId = "" }},
		{"missing table", func(r *v1_orderpb.CreateOrderRequest) { r.TableId = "" }},
		{"missing guest session", func(r *v1_orderpb.CreateOrderRequest) { r.GuestSessionId = "" }},
		{"no items", func(r *v1_orderpb.CreateOrderRequest) { r.Items = nil }},
		{"missing item id", func(r *v1_orderpb.CreateOrderRequest) { r.Items[0].ItemId = "" }},
		{"qty zero", func(r *v1_orderpb.CreateOrderRequest) { r.Items[0].Qty = 0 }},
		{"qty negative", func(r *v1_orderpb.CreateOrderRequest) { r.Items[0].Qty = -1 }},
		{"qty above 99", func(r *v1_orderpb.CreateOrderRequest) { r.Items[0].Qty = 100 }},
		{"too many lines", func(r *v1_orderpb.CreateOrderRequest) {
			r.Items = make([]*v1_orderpb.OrderItemRequest, venue.MaxItemsPerOrder+1)
			for i := range r.Items {
				r.Items[i] = &v1_orderpb.OrderItemRequest{ItemId: "item", Qty: 1}
			}
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := valid()
			tc.mutate(req)
			err := validateCreateOrder(req)
			if err == nil {
				t.Fatal("expected a validation error, got nil")
			}
			if got := codeOf(err); got != codes.InvalidArgument {
				t.Errorf("code = %v, want InvalidArgument", got)
			}
		})
	}

	// Exactly at the cap is allowed; the check is > not >=.
	atCap := valid()
	atCap.Items = make([]*v1_orderpb.OrderItemRequest, venue.MaxItemsPerOrder)
	for i := range atCap.Items {
		atCap.Items[i] = &v1_orderpb.OrderItemRequest{ItemId: "item", Qty: 1}
	}
	if err := validateCreateOrder(atCap); err != nil {
		t.Errorf("an order of exactly %d lines should be allowed: %v", venue.MaxItemsPerOrder, err)
	}

	// Both ends of the qty range are inclusive.
	for _, qty := range []int32{1, 99} {
		req := valid()
		req.Items[0].Qty = qty
		if err := validateCreateOrder(req); err != nil {
			t.Errorf("qty %d should be allowed: %v", qty, err)
		}
	}
}

// TestValidateCommentsCountsRunes guards the specific bug the function's
// comment calls out: counting bytes would reject a comment well inside
// the character limit for any non-ASCII script.
func TestValidateCommentsCountsRunes(t *testing.T) {
	req := &v1_orderpb.CreateOrderRequest{
		Items: []*v1_orderpb.OrderItemRequest{
			{ItemId: "item", Qty: 1, Comment: strings.Repeat("辛", 100)},
		},
	}
	if err := validateComments(req, 200); err != nil {
		t.Errorf("100 multi-byte characters should pass a 200-character limit: %v", err)
	}

	req.Items[0].Comment = strings.Repeat("辛", 201)
	if err := validateComments(req, 200); err == nil {
		t.Error("201 characters should fail a 200-character limit")
	} else if got := codeOf(err); got != codes.InvalidArgument {
		t.Errorf("code = %v, want InvalidArgument", got)
	}

	// Exactly at the limit is allowed.
	req.Items[0].Comment = strings.Repeat("a", 200)
	if err := validateComments(req, 200); err != nil {
		t.Errorf("a comment of exactly the limit should pass: %v", err)
	}
}

func TestCreateOrderFingerprint(t *testing.T) {
	base := &v1_orderpb.CreateOrderRequest{
		VenueId:        "v1",
		TableId:        "t1",
		GuestSessionId: "g1",
		Locale:         "en",
		Items: []*v1_orderpb.OrderItemRequest{
			{ItemId: "i1", Qty: 2, ModifierOptionIds: []string{"a", "b"}, Comment: "no ice"},
		},
	}

	t.Run("is stable", func(t *testing.T) {
		first, second := createOrderFingerprint(base), createOrderFingerprint(base)
		if first != second {
			t.Errorf("the same request hashed differently twice: %s vs %s", first, second)
		}
	})

	t.Run("modifier order does not matter", func(t *testing.T) {
		reordered := &v1_orderpb.CreateOrderRequest{
			VenueId: "v1", TableId: "t1", GuestSessionId: "g1", Locale: "en",
			Items: []*v1_orderpb.OrderItemRequest{
				{ItemId: "i1", Qty: 2, ModifierOptionIds: []string{"b", "a"}, Comment: "no ice"},
			},
		}
		if createOrderFingerprint(base) != createOrderFingerprint(reordered) {
			t.Error("the same modifier selection in a different order should share a fingerprint")
		}
	})

	t.Run("distinguishes every meaningful field", func(t *testing.T) {
		mutations := map[string]func(*v1_orderpb.CreateOrderRequest){
			"venue":    func(r *v1_orderpb.CreateOrderRequest) { r.VenueId = "v2" },
			"table":    func(r *v1_orderpb.CreateOrderRequest) { r.TableId = "t2" },
			"guest":    func(r *v1_orderpb.CreateOrderRequest) { r.GuestSessionId = "g2" },
			"locale":   func(r *v1_orderpb.CreateOrderRequest) { r.Locale = "ru" },
			"item":     func(r *v1_orderpb.CreateOrderRequest) { r.Items[0].ItemId = "i2" },
			"qty":      func(r *v1_orderpb.CreateOrderRequest) { r.Items[0].Qty = 3 },
			"modifier": func(r *v1_orderpb.CreateOrderRequest) { r.Items[0].ModifierOptionIds = []string{"a", "c"} },
			"comment":  func(r *v1_orderpb.CreateOrderRequest) { r.Items[0].Comment = "extra ice" },
			"extra line": func(r *v1_orderpb.CreateOrderRequest) {
				r.Items = append(r.Items, &v1_orderpb.OrderItemRequest{ItemId: "i9", Qty: 1})
			},
		}
		want := createOrderFingerprint(base)
		for name, mutate := range mutations {
			t.Run(name, func(t *testing.T) {
				mutated := &v1_orderpb.CreateOrderRequest{
					VenueId: base.VenueId, TableId: base.TableId,
					GuestSessionId: base.GuestSessionId, Locale: base.Locale,
					Items: []*v1_orderpb.OrderItemRequest{{
						ItemId:            base.Items[0].ItemId,
						Qty:               base.Items[0].Qty,
						ModifierOptionIds: append([]string(nil), base.Items[0].ModifierOptionIds...),
						Comment:           base.Items[0].Comment,
					}},
				}
				mutate(mutated)
				if createOrderFingerprint(mutated) == want {
					t.Errorf("changing the %s did not change the fingerprint", name)
				}
			})
		}
	})

	t.Run("field boundaries are separated", func(t *testing.T) {
		// Without a domain separator in Fingerprint, ("ab","c") and
		// ("a","bc") would hash identically — and two different carts
		// sharing a fingerprint means a replayed response for the wrong
		// order.
		a := &v1_orderpb.CreateOrderRequest{VenueId: "ab", TableId: "c", GuestSessionId: "g"}
		b := &v1_orderpb.CreateOrderRequest{VenueId: "a", TableId: "bc", GuestSessionId: "g"}
		if createOrderFingerprint(a) == createOrderFingerprint(b) {
			t.Error("adjacent fields ran together in the hash")
		}
	})
}

func TestClampPageSize(t *testing.T) {
	cases := []struct {
		in   int32
		want int
	}{
		{0, defaultPageSize},
		{-1, defaultPageSize},
		{1, 1},
		{50, 50},
		{maxPageSize, maxPageSize},
		{maxPageSize + 1, maxPageSize},
		{1 << 20, maxPageSize},
	}
	for _, tc := range cases {
		if got := clampPageSize(tc.in); got != tc.want {
			t.Errorf("clampPageSize(%d) = %d, want %d", tc.in, got, tc.want)
		}
	}
}

// TestEventPayloadsCarryTheirRoutingFields guards a failure mode that is
// invisible in the order service and only shows up two hops away: the
// gateway routes an event to the guests' WebSocket channel by reading
// table_session_id out of the payload, so an event that omits it reaches
// the kitchen and nobody else.
//
// That is exactly what happened to order.item_transitioned — a line going
// "ready" never reached the guest, and it took an end-to-end run to
// notice, because every other order event happened to carry the field.
// This test asserts the shape directly so the next omission fails here.
func TestEventPayloadsCarryTheirRoutingFields(t *testing.T) {
	const sessionID = "11111111-1111-1111-1111-111111111111"

	// Every payload the gateway routes to a guest session channel; see
	// services/gateway/internal/consumer.Route.
	guestVisible := []struct {
		name    string
		payload any
	}{
		{eventOrderPlaced, orderPlacedPayload{TableSessionID: sessionID}},
		{eventOrderTransitioned, orderTransitionedPayload{TableSessionID: sessionID}},
		{eventOrderItemTransitioned, orderItemTransitionedPayload{TableSessionID: sessionID}},
		{eventTableSessionOpened, tableSessionOpenedPayload{TableSessionID: sessionID}},
		{eventTableSessionClosed, tableSessionClosedPayload{TableSessionID: sessionID}},
		{eventServiceRequestCreated, serviceRequestCreatedPayload{TableSessionID: sessionID}},
		{eventServiceRequestTransited, serviceRequestTransitionedPayload{TableSessionID: sessionID}},
	}

	for _, tc := range guestVisible {
		t.Run(tc.name, func(t *testing.T) {
			raw, err := json.Marshal(tc.payload)
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}
			var decoded struct {
				TableSessionID string `json:"table_session_id"`
			}
			if err := json.Unmarshal(raw, &decoded); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			if decoded.TableSessionID != sessionID {
				t.Errorf("%s payload has no table_session_id — the gateway cannot route it to the guest: %s",
					tc.name, raw)
			}
		})
	}
}
