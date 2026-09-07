package consumer

import (
	"encoding/json"
	"testing"

	"github.com/menli02/QR-menu/services/gateway/internal/ws"
)

func routeSet(eventType, venueID string, payload string) map[string]bool {
	got := Route(eventType, venueID, json.RawMessage(payload))
	out := make(map[string]bool, len(got))
	for _, c := range got {
		out[c] = true
	}
	return out
}

// TestRouteOrderEvents: the kitchen needs every order event (FR-K2), the
// party at that table needs its own (FR-O10). Both, not either.
func TestRouteOrderEvents(t *testing.T) {
	payload := `{"order_id":"o1","table_id":"t1","table_session_id":"s1"}`

	for _, eventType := range []string{
		"order.placed", "order.transitioned", "order.item_transitioned", "order.cancelled",
	} {
		t.Run(eventType, func(t *testing.T) {
			got := routeSet(eventType, "v1", payload)
			if !got[ws.VenueKDSChannel("v1")] {
				t.Error("not routed to the kitchen")
			}
			if !got[ws.SessionChannel("s1")] {
				t.Error("not routed to the guest's session")
			}
			if got[ws.VenueMenuChannel("v1")] {
				t.Error("an order event has no business on the menu channel")
			}
			if len(got) != 2 {
				t.Errorf("routed to %d channels, want 2", len(got))
			}
		})
	}
}

// TestRouteCatalogEvents is FR-C4's five-second stop-list promise: guest
// menus and kitchen screens both need to know an item was 86'd.
func TestRouteCatalogEvents(t *testing.T) {
	for _, eventType := range []string{"catalog.item_availability_changed", "catalog.menu_published"} {
		t.Run(eventType, func(t *testing.T) {
			got := routeSet(eventType, "v1", `{"item_id":"i1","is_available":false}`)
			if !got[ws.VenueMenuChannel("v1")] {
				t.Error("not routed to the guest menu channel")
			}
			if !got[ws.VenueKDSChannel("v1")] {
				t.Error("not routed to the kitchen — a cook should see what another station just 86'd")
			}
		})
	}
}

func TestRouteServiceRequestEvents(t *testing.T) {
	payload := `{"service_request_id":"r1","table_id":"t1","table_session_id":"s1","type":"call_waiter"}`

	for _, eventType := range []string{"service_request.created", "service_request.transitioned"} {
		t.Run(eventType, func(t *testing.T) {
			got := routeSet(eventType, "v1", payload)
			if !got[ws.VenueFloorChannel("v1")] {
				t.Error("floor staff were not notified")
			}
			if !got[ws.SessionChannel("s1")] {
				t.Error("the guest was not told their request was seen")
			}
		})
	}
}

func TestRouteTableSessionEvents(t *testing.T) {
	payload := `{"table_session_id":"s1","table_id":"t1"}`

	for _, eventType := range []string{"table_session.opened", "table_session.closed"} {
		t.Run(eventType, func(t *testing.T) {
			got := routeSet(eventType, "v1", payload)
			if !got[ws.VenueFloorChannel("v1")] {
				t.Error("the floor view tracks occupancy and was not notified")
			}
			if !got[ws.SessionChannel("s1")] {
				t.Error("the guests were not told their session changed")
			}
		})
	}
}

// TestRouteUnknownEventTypeIsIgnored covers §8.3's additive-change rule: an
// older gateway meeting a newer producer must tolerate the event, not
// crash or spam.
func TestRouteUnknownEventTypeIsIgnored(t *testing.T) {
	if got := Route("order.reticulated", "v1", json.RawMessage(`{}`)); len(got) != 0 {
		t.Errorf("unknown event routed to %v, want nowhere", got)
	}
}

// TestRouteToleratesMissingRoutingFields: a payload without a session id
// must still reach the channels that don't need one, rather than being
// dropped entirely.
func TestRouteToleratesMissingRoutingFields(t *testing.T) {
	got := routeSet("order.placed", "v1", `{"order_id":"o1"}`)
	if !got[ws.VenueKDSChannel("v1")] {
		t.Error("the kitchen route was lost because the payload had no session id")
	}
	if len(got) != 1 {
		t.Errorf("routed to %v, want only the kitchen", got)
	}
}

func TestRouteToleratesUnparseablePayload(t *testing.T) {
	// A payload the router can't read should still reach the channels that
	// depend only on the event type and venue.
	got := routeSet("catalog.item_availability_changed", "v1", `not json`)
	if !got[ws.VenueMenuChannel("v1")] {
		t.Error("a bad payload lost a route that didn't depend on it")
	}
}

// TestRouteNeverCrossesVenues is the isolation property, checked at the
// routing layer rather than only at the hub.
func TestRouteNeverCrossesVenues(t *testing.T) {
	got := Route("order.placed", "venue-a", json.RawMessage(`{"table_session_id":"s1"}`))
	for _, c := range got {
		if c == ws.VenueKDSChannel("venue-b") || c == ws.VenueMenuChannel("venue-b") {
			t.Fatalf("event for venue-a routed to venue-b: %v", got)
		}
	}
}

func TestConsumerWithoutBrokersIsANoOp(t *testing.T) {
	hub := ws.NewHub()
	c := New(hub, Config{})
	if c.reader != nil {
		t.Fatal("a consumer with no brokers should have no reader")
	}

	done := make(chan struct{})
	go func() { c.Start(); close(done) }()
	<-done // must return immediately rather than dialling nothing forever
	c.Stop()
}

func TestConsumerDefaults(t *testing.T) {
	c := New(ws.NewHub(), Config{})
	if len(c.cfg.Topics) != 3 {
		t.Errorf("topics = %v, want the three §8.3 topics", c.cfg.Topics)
	}
	if c.cfg.GroupPrefix != "gateway" {
		t.Errorf("group prefix = %q, want gateway", c.cfg.GroupPrefix)
	}
	if c.cfg.InstanceID == "" {
		t.Error("instance id is empty — two pods would share a consumer group and split events between them")
	}
}

func TestSanitizeGroupID(t *testing.T) {
	cases := map[string]string{
		"gateway-abc123":      "gateway-abc123",
		"pod.name_1":          "pod.name_1",
		"host/with:bad chars": "host-with-bad-chars",
		"UPPER":               "UPPER",
	}
	for in, want := range cases {
		if got := sanitize(in); got != want {
			t.Errorf("sanitize(%q) = %q, want %q", in, got, want)
		}
	}
}

// TestHandleFansOutToTheHub covers the consumer's own decode-and-dispatch
// step, which Route alone does not.
func TestHandleFansOutToTheHub(t *testing.T) {
	hub := ws.NewHub()
	c := New(hub, Config{})

	envelopeJSON := `{
		"event_id":"e1",
		"event_type":"order.placed",
		"schema_version":1,
		"venue_id":"v1",
		"occurred_at":"2026-03-01T10:00:00Z",
		"payload":{"order_id":"o1","table_session_id":"s1"}
	}`

	// No subscribers: this must not panic or block.
	c.handle(kafkaMessage(envelopeJSON))

	if counts := hub.Counts(); len(counts) != 0 {
		t.Errorf("handling created channels: %v", counts)
	}
}

func TestHandleIgnoresUndecodableAndUnroutableMessages(t *testing.T) {
	c := New(ws.NewHub(), Config{})

	// Neither of these should panic: a message this consumer cannot use
	// must not stall every event behind it.
	c.handle(kafkaMessage(`not json at all`))
	c.handle(kafkaMessage(`{"event_id":"e1","event_type":"order.placed"}`)) // no venue_id
}
