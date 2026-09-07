package ws

import (
	"encoding/json"
	"testing"
)

// newTestClient makes a Client with no socket behind it. Every hub
// behaviour worth testing — subscription bookkeeping, fan-out, the
// slow-consumer drop — is decided before a byte reaches the network, so
// the send channel is the only part that has to be real.
func newTestClient(buffer int) *Client {
	return &Client{send: make(chan []byte, buffer), label: "test"}
}

func drain(t *testing.T, c *Client) []Event {
	t.Helper()
	var out []Event
	for {
		select {
		case raw := <-c.send:
			var ev Event
			if err := json.Unmarshal(raw, &ev); err != nil {
				t.Fatalf("client received unparseable JSON: %v", err)
			}
			out = append(out, ev)
		default:
			return out
		}
	}
}

func TestChannelNames(t *testing.T) {
	// The names are a contract with the clients (docs/TZ.md §7.3), so they
	// are pinned rather than derived.
	if got := VenueKDSChannel("v1"); got != "venue:v1:kds" {
		t.Errorf("VenueKDSChannel = %q", got)
	}
	if got := VenueMenuChannel("v1"); got != "venue:v1:menu" {
		t.Errorf("VenueMenuChannel = %q", got)
	}
	if got := SessionChannel("s1"); got != "session:s1" {
		t.Errorf("SessionChannel = %q", got)
	}
	if got := VenueFloorChannel("v1"); got != "venue:v1:floor" {
		t.Errorf("VenueFloorChannel = %q", got)
	}
}

func TestBroadcastReachesOnlySubscribers(t *testing.T) {
	hub := NewHub()
	kds := newTestClient(4)
	guest := newTestClient(4)

	hub.Subscribe(kds, VenueKDSChannel("v1"))
	hub.Subscribe(guest, SessionChannel("s1"))

	hub.Broadcast(VenueKDSChannel("v1"), Event{EventID: "e1", EventType: "order.placed"})

	got := drain(t, kds)
	if len(got) != 1 || got[0].EventID != "e1" {
		t.Fatalf("kds received %v, want one event", got)
	}
	if got[0].Channel != VenueKDSChannel("v1") {
		t.Errorf("channel = %q, want it stamped on the event", got[0].Channel)
	}
	if len(drain(t, guest)) != 0 {
		t.Error("a guest on another channel received the KDS event")
	}
}

// TestVenueIsolation is the property that keeps one venue's tickets off
// another's screens. Channel names embed the venue id, so this is really a
// test that nothing collapses them.
func TestVenueIsolation(t *testing.T) {
	hub := NewHub()
	a := newTestClient(4)
	b := newTestClient(4)
	hub.Subscribe(a, VenueKDSChannel("venue-a"))
	hub.Subscribe(b, VenueKDSChannel("venue-b"))

	hub.Broadcast(VenueKDSChannel("venue-a"), Event{EventID: "e1"})

	if len(drain(t, a)) != 1 {
		t.Error("venue-a did not receive its own event")
	}
	if got := drain(t, b); len(got) != 0 {
		t.Errorf("venue-b received venue-a's event: %v", got)
	}
}

func TestMultipleSubscribersAllReceive(t *testing.T) {
	hub := NewHub()
	clients := make([]*Client, 5)
	for i := range clients {
		clients[i] = newTestClient(4)
		hub.Subscribe(clients[i], VenueKDSChannel("v1"))
	}

	hub.Broadcast(VenueKDSChannel("v1"), Event{EventID: "e1"})

	for i, c := range clients {
		if len(drain(t, c)) != 1 {
			t.Errorf("client %d did not receive the broadcast", i)
		}
	}
}

// TestSlowConsumerIsDroppedNotWaitedFor is the decision that keeps one
// wedged browser tab from stalling the Kafka consumer and every other
// screen in the venue.
func TestSlowConsumerIsDroppedNotWaitedFor(t *testing.T) {
	hub := NewHub()
	slow := newTestClient(1) // fills after one message
	fast := newTestClient(8)
	hub.Subscribe(slow, VenueKDSChannel("v1"))
	hub.Subscribe(fast, VenueKDSChannel("v1"))

	// Three broadcasts; the slow client can hold one.
	for i := 0; i < 3; i++ {
		hub.Broadcast(VenueKDSChannel("v1"), Event{EventID: "e"})
	}

	if got := len(drain(t, slow)); got != 1 {
		t.Errorf("slow client holds %d messages, want 1 (the rest dropped)", got)
	}
	if got := len(drain(t, fast)); got != 3 {
		t.Errorf("fast client received %d, want all 3 — a slow peer must not cost it messages", got)
	}
}

func TestUnsubscribeRemovesFromEveryChannel(t *testing.T) {
	hub := NewHub()
	c := newTestClient(4)
	hub.Subscribe(c, VenueKDSChannel("v1"))
	hub.Subscribe(c, VenueMenuChannel("v1"))
	hub.Subscribe(c, SessionChannel("s1"))

	if got := len(hub.Counts()); got != 3 {
		t.Fatalf("hub tracks %d channels, want 3", got)
	}

	hub.Unsubscribe(c)

	// Empty channels must be reclaimed, not left as empty maps: names are
	// per-venue and per-session, so they accumulate without bound over a
	// long service otherwise.
	if got := hub.Counts(); len(got) != 0 {
		t.Errorf("channels remain after unsubscribe: %v", got)
	}
}

func TestUnsubscribeLeavesOtherClients(t *testing.T) {
	hub := NewHub()
	a, b := newTestClient(4), newTestClient(4)
	hub.Subscribe(a, VenueKDSChannel("v1"))
	hub.Subscribe(b, VenueKDSChannel("v1"))

	hub.Unsubscribe(a)

	if got := hub.Counts()[VenueKDSChannel("v1")]; got != 1 {
		t.Errorf("channel holds %d clients, want 1", got)
	}
	hub.Broadcast(VenueKDSChannel("v1"), Event{EventID: "e1"})
	if len(drain(t, b)) != 1 {
		t.Error("the remaining client stopped receiving")
	}
}

func TestBroadcastToEmptyChannelIsHarmless(t *testing.T) {
	hub := NewHub()
	hub.Broadcast(VenueKDSChannel("nobody"), Event{EventID: "e1"})
	if got := len(hub.Counts()); got != 0 {
		t.Errorf("broadcasting created %d channels, want 0", got)
	}
}

// TestPayloadIsPassedThroughVerbatim matters because the gateway is a
// relay here, not an interpreter: re-encoding a payload would be a second
// place for the event schema to drift from §8.3.
func TestPayloadIsPassedThroughVerbatim(t *testing.T) {
	hub := NewHub()
	c := newTestClient(4)
	hub.Subscribe(c, VenueKDSChannel("v1"))

	payload := json.RawMessage(`{"order_id":"o1","total_minor":900,"nested":{"a":[1,2]}}`)
	hub.Broadcast(VenueKDSChannel("v1"), Event{EventID: "e1", EventType: "order.placed", Payload: payload})

	got := drain(t, c)
	if len(got) != 1 {
		t.Fatalf("received %d events", len(got))
	}
	var round map[string]any
	if err := json.Unmarshal(got[0].Payload, &round); err != nil {
		t.Fatalf("payload is not valid JSON after the round trip: %v", err)
	}
	if round["order_id"] != "o1" {
		t.Errorf("payload lost fields: %v", round)
	}
}

func TestConcurrentSubscribeAndBroadcast(t *testing.T) {
	// Run with -race: the hub is touched by the Kafka consumer goroutine
	// and by every connection's handler at once.
	hub := NewHub()
	done := make(chan struct{})

	go func() {
		for i := 0; i < 200; i++ {
			hub.Broadcast(VenueKDSChannel("v1"), Event{EventID: "e"})
		}
		close(done)
	}()

	for i := 0; i < 50; i++ {
		c := newTestClient(1)
		hub.Subscribe(c, VenueKDSChannel("v1"))
		hub.Unsubscribe(c)
	}
	<-done
}
