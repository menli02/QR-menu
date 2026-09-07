// Package consumer bridges Kafka to the WebSocket hub: it reads the
// domain events the services publish and pushes them to the sockets this
// gateway instance holds (docs/TZ.md §7.3).
//
// The consumer-group choice is the whole design, so it is worth stating
// plainly. Every gateway pod joins with its *own* group id
// ("gateway-<instance>"), which means every pod receives every event —
// the opposite of the usual load-sharing arrangement. That is deliberate:
// a socket lives on exactly one pod, and a pod can only push to sockets it
// holds, so an event delivered to just one pod would reach only the
// clients that happen to be connected there. §7.3 makes the same call and
// notes the trade: these consumers are read-only and idempotent, so
// receiving an event N times across N pods is free.
package consumer

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"time"

	"github.com/menli02/QR-menu/services/gateway/internal/ws"

	"github.com/segmentio/kafka-go"
	"github.com/zeromicro/go-zero/core/logx"
)

// Topics this gateway consumes, per docs/TZ.md §8.3.
var defaultTopics = []string{
	"qrmenu.order.v1",
	"qrmenu.service_request.v1",
	"qrmenu.catalog.v1",
}

type Config struct {
	Brokers []string
	Topics  []string
	// GroupPrefix plus the instance id forms the consumer group. Defaults
	// to "gateway".
	GroupPrefix string
	// InstanceID distinguishes this pod's group from its siblings'.
	// Defaults to HOSTNAME, which Kubernetes sets to the pod name.
	InstanceID string
}

// envelope is the §8.3 message format the outbox relay writes.
type envelope struct {
	EventID    string          `json:"event_id"`
	EventType  string          `json:"event_type"`
	OccurredAt time.Time       `json:"occurred_at"`
	VenueID    string          `json:"venue_id"`
	TraceID    string          `json:"trace_id"`
	Payload    json.RawMessage `json:"payload"`
}

// payloadRouting is the subset of a payload the router needs to decide
// which channels an event belongs on. Every field is optional — different
// event types carry different ones — so this is a lenient partial decode
// rather than a schema.
type payloadRouting struct {
	TableSessionID string `json:"table_session_id"`
	TableID        string `json:"table_id"`
	ItemID         string `json:"item_id"`
}

type Consumer struct {
	hub    *ws.Hub
	reader *kafka.Reader
	cfg    Config

	cancel context.CancelFunc
	done   chan struct{}
}

// New builds a consumer. With no brokers configured it returns one whose
// Start is a no-op, so local development without Kafka runs the gateway
// unchanged — REST works, sockets connect, nothing is pushed.
func New(hub *ws.Hub, cfg Config) *Consumer {
	if len(cfg.Topics) == 0 {
		cfg.Topics = defaultTopics
	}
	if cfg.GroupPrefix == "" {
		cfg.GroupPrefix = "gateway"
	}
	if cfg.InstanceID == "" {
		cfg.InstanceID = instanceID()
	}

	c := &Consumer{hub: hub, cfg: cfg, done: make(chan struct{})}
	if len(cfg.Brokers) == 0 {
		return c
	}

	c.reader = kafka.NewReader(kafka.ReaderConfig{
		Brokers:     cfg.Brokers,
		GroupID:     cfg.GroupPrefix + "-" + cfg.InstanceID,
		GroupTopics: cfg.Topics,
		MinBytes:    1,
		MaxBytes:    10 << 20,
		// A socket push that arrives a second late is useless, so latency
		// beats batching here.
		MaxWait: 250 * time.Millisecond,
		// StartOffset applies only to a group with no committed offset —
		// which, with per-pod groups, is every pod on its first start.
		// LastOffset means a new pod pushes what happens *from now*
		// rather than replaying a week of history to a KDS screen.
		StartOffset: kafka.LastOffset,
	})
	return c
}

// Start runs the consume loop until Stop. It blocks, matching go-zero's
// service.Service contract.
func (c *Consumer) Start() {
	defer close(c.done)

	if c.reader == nil {
		logx.Info("kafka consumer disabled: no brokers configured, realtime push is off")
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	c.cancel = cancel
	logx.Infof("kafka consumer started as group %s on %v", c.reader.Config().GroupID, c.cfg.Topics)

	for {
		msg, err := c.reader.FetchMessage(ctx)
		if err != nil {
			if errors.Is(err, context.Canceled) {
				return
			}
			logx.Errorf("kafka fetch failed: %v", err)
			// Back off rather than spinning on a broker that is down.
			select {
			case <-ctx.Done():
				return
			case <-time.After(time.Second):
			}
			continue
		}

		c.handle(msg)

		// Commit after handling. With at-most-once push semantics this is
		// the right order: a crash between handling and committing
		// re-delivers an event the sockets already saw, and clients dedupe
		// by eventId. Committing first would silently drop it instead.
		if err := c.reader.CommitMessages(ctx, msg); err != nil && !errors.Is(err, context.Canceled) {
			logx.Errorf("kafka commit failed: %v", err)
		}
	}
}

func (c *Consumer) Stop() {
	if c.cancel != nil {
		c.cancel()
	}
	<-c.done
	if c.reader != nil {
		if err := c.reader.Close(); err != nil {
			logx.Errorf("kafka reader close failed: %v", err)
		}
	}
}

func (c *Consumer) handle(msg kafka.Message) {
	var env envelope
	if err := json.Unmarshal(msg.Value, &env); err != nil {
		// A message this consumer cannot parse is not worth stalling on:
		// it is read-only fan-out, and the durable record is in Kafka
		// either way. Log and move on rather than blocking every
		// subsequent event behind it.
		logx.Errorf("kafka: undecodable message on %s offset %d: %v", msg.Topic, msg.Offset, err)
		return
	}
	if env.VenueID == "" {
		logx.Errorf("kafka: event %s has no venue_id, cannot route", env.EventID)
		return
	}

	event := ws.Event{EventID: env.EventID, EventType: env.EventType, Payload: env.Payload}
	for _, channel := range Route(env.EventType, env.VenueID, env.Payload) {
		c.hub.Broadcast(channel, event)
	}
}

// Route decides which WebSocket channels an event belongs on.
//
// Kept as a pure function so the routing table — the part most likely to
// be got wrong, and least likely to be noticed when it is — can be tested
// without a broker or a socket.
func Route(eventType, venueID string, payload json.RawMessage) []string {
	var p payloadRouting
	_ = json.Unmarshal(payload, &p) // partial decode; absent fields stay empty

	var channels []string
	addSession := func() {
		if p.TableSessionID != "" {
			channels = append(channels, ws.SessionChannel(p.TableSessionID))
		}
	}

	switch eventType {
	case "order.placed", "order.transitioned", "order.item_transitioned", "order.cancelled":
		// The kitchen needs every order event; the guests at that table
		// need their own. Both, not either: FR-K2 pushes new tickets to
		// KDS and FR-O10 shows the guest live status.
		channels = append(channels, ws.VenueKDSChannel(venueID))
		addSession()

	case "table_session.opened", "table_session.closed":
		// The floor view tracks table occupancy; the guests need to know
		// their session ended (their token stops working with it).
		channels = append(channels, ws.VenueFloorChannel(venueID))
		addSession()

	case "service_request.created", "service_request.transitioned":
		// Floor staff act on these. Guests see their own request being
		// acknowledged, which is the entire point of the feature.
		channels = append(channels, ws.VenueFloorChannel(venueID))
		addSession()

	case "catalog.item_availability_changed", "catalog.menu_published":
		// FR-C4: a stop-list change must reach guest menus within 5s. KDS
		// too, so a cook sees an item another station just 86'd.
		channels = append(channels, ws.VenueMenuChannel(venueID), ws.VenueKDSChannel(venueID))

	default:
		// An event type this gateway does not know about is not an error:
		// §8.3 allows producers to add types additively, and an older
		// gateway must tolerate a newer producer rather than log-spam or
		// crash. It simply isn't routed anywhere.
		logx.Infof("kafka: no route for event type %q, ignoring", eventType)
	}

	return channels
}

// instanceID identifies this pod. HOSTNAME is the pod name under
// Kubernetes, which is exactly the per-instance identity §7.3 wants; the
// timestamp fallback keeps two local processes from sharing a group and
// silently splitting events between them.
func instanceID() string {
	if h, err := os.Hostname(); err == nil && h != "" {
		return sanitize(h)
	}
	return "local-" + time.Now().UTC().Format("20060102150405")
}

// sanitize strips characters Kafka rejects in a group id.
func sanitize(s string) string {
	return strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_', r == '.':
			return r
		default:
			return '-'
		}
	}, s)
}
