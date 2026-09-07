// Package ws is the gateway's realtime fan-out: the WebSocket hub that
// pushes domain events to KDS screens and guest phones (docs/TZ.md §7.3).
//
// Its place in the system is worth being precise about, because it shapes
// every decision here: sockets are an *optimisation*, never the source of
// truth. Clients re-fetch state over REST on reconnect and fall back to
// polling if the socket fails twice. So the hub is allowed to drop a
// message to a slow consumer, and is not allowed to block a Kafka consumer
// or a request handler while it decides.
package ws

import (
	"encoding/json"
	"sync"

	"github.com/zeromicro/go-zero/core/logx"
)

// Channel names, per docs/TZ.md §7.3.
//
// They are functions rather than format strings at the call site so the
// naming lives in one place — a typo in a channel name is a silent
// delivery failure, not an error.
func VenueKDSChannel(venueID string) string   { return "venue:" + venueID + ":kds" }
func VenueMenuChannel(venueID string) string  { return "venue:" + venueID + ":menu" }
func SessionChannel(sessionID string) string  { return "session:" + sessionID }
func VenueFloorChannel(venueID string) string { return "venue:" + venueID + ":floor" }

// Event is what a client receives. It mirrors the Kafka envelope's
// identifying fields (docs/TZ.md §8.3) so a client can dedupe by event_id
// across a reconnect — at-least-once delivery upstream means the same
// event can legitimately arrive twice.
type Event struct {
	EventID   string          `json:"eventId"`
	EventType string          `json:"eventType"`
	Channel   string          `json:"channel"`
	Payload   json.RawMessage `json:"payload"`
}

// Hub tracks which connections are listening on which channels.
//
// One mutex over a map of maps, deliberately: a venue has tens of sockets,
// not thousands, and broadcast is a map lookup plus a handful of
// non-blocking channel sends. Sharding this would be complexity bought
// against a load that does not exist.
type Hub struct {
	mu       sync.RWMutex
	channels map[string]map[*Client]struct{}
}

func NewHub() *Hub {
	return &Hub{channels: make(map[string]map[*Client]struct{})}
}

// Subscribe adds a client to a channel. Authorisation happens before this
// is called — see handler.go's authorise — so the hub itself trusts its
// caller and does no checking of its own.
func (h *Hub) Subscribe(c *Client, channel string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	subs, ok := h.channels[channel]
	if !ok {
		subs = make(map[*Client]struct{})
		h.channels[channel] = subs
	}
	subs[c] = struct{}{}
}

// Unsubscribe removes a client from every channel it holds. Called once,
// on disconnect; there is no per-channel unsubscribe because no client
// needs one — a KDS screen watches its venue for as long as it is open.
func (h *Hub) Unsubscribe(c *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for name, subs := range h.channels {
		if _, ok := subs[c]; !ok {
			continue
		}
		delete(subs, c)
		if len(subs) == 0 {
			// Reclaim the map: channel names are per-venue and per-table
			// session, so an unbounded set of them accumulates over a long
			// service otherwise.
			delete(h.channels, name)
		}
	}
}

// Broadcast delivers an event to every subscriber of a channel.
//
// Delivery is best-effort by design. Client.send is buffered, and a client
// whose buffer is full is *skipped*, not waited for. The alternative —
// blocking — would let one wedged browser tab stall the Kafka consumer and
// with it every other screen in the venue. A client that misses a message
// re-fetches over REST on its next reconnect, which is exactly the
// contract §7.3 sets.
func (h *Hub) Broadcast(channel string, ev Event) {
	ev.Channel = channel

	h.mu.RLock()
	subs := make([]*Client, 0, len(h.channels[channel]))
	for c := range h.channels[channel] {
		subs = append(subs, c)
	}
	h.mu.RUnlock()

	if len(subs) == 0 {
		return
	}

	data, err := json.Marshal(ev)
	if err != nil {
		logx.Errorf("ws: cannot encode event %s: %v", ev.EventType, err)
		return
	}

	var dropped int
	for _, c := range subs {
		select {
		case c.send <- data:
		default:
			dropped++
		}
	}
	if dropped > 0 {
		logx.Errorf("ws: dropped %s for %d slow subscriber(s) on %s", ev.EventType, dropped, channel)
	}
}

// Counts reports subscribers per channel, for the readiness/metrics
// surface and for tests.
func (h *Hub) Counts() map[string]int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	out := make(map[string]int, len(h.channels))
	for name, subs := range h.channels {
		out[name] = len(subs)
	}
	return out
}
