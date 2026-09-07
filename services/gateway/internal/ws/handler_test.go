package ws

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// ---------------------------------------------------------------------
// Fakes
// ---------------------------------------------------------------------

type fakeStaffVerifier struct {
	identity StaffIdentity
	err      error
}

func (f fakeStaffVerifier) Verify(context.Context, string) (StaffIdentity, error) {
	return f.identity, f.err
}

type fakeGuestVerifier struct {
	identity GuestIdentity
	err      error
}

func (f fakeGuestVerifier) Verify(context.Context, string) (GuestIdentity, error) {
	return f.identity, f.err
}

type fakeSessions struct {
	id  string
	err error
}

func (f fakeSessions) CurrentSessionID(context.Context, string, string) (string, error) {
	return f.id, f.err
}

// dial starts a server around h and opens a client connection to path.
func dial(t *testing.T, h *Handler, path string) *websocket.Conn {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/ws/guest", h.ServeGuest)
	mux.HandleFunc("/ws/staff", h.ServeStaff)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	conn, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(srv.URL, "http")+path, nil)
	if err != nil {
		t.Fatalf("dial %s: %v", path, err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	return conn
}

// readFrame reads one JSON frame with a deadline, so a hung test fails
// fast instead of blocking the suite.
func readFrame(t *testing.T, conn *websocket.Conn, v any) error {
	t.Helper()
	if err := conn.SetReadDeadline(time.Now().Add(3 * time.Second)); err != nil {
		return err
	}
	return conn.ReadJSON(v)
}

// ---------------------------------------------------------------------
// Guest
// ---------------------------------------------------------------------

func TestGuestSubscribesToOwnSessionAndVenueMenu(t *testing.T) {
	hub := NewHub()
	h := NewHandler(hub,
		fakeStaffVerifier{},
		fakeGuestVerifier{identity: GuestIdentity{VenueID: "v1", TableID: "t1", GuestSessionID: "g1"}},
		fakeSessions{id: "s1"},
		nil)

	conn := dial(t, h, "/ws/guest")
	if err := conn.WriteJSON(subscribeFrame{Token: "good"}); err != nil {
		t.Fatalf("send subscribe: %v", err)
	}

	var ack ackFrame
	if err := readFrame(t, conn, &ack); err != nil {
		t.Fatalf("read ack: %v", err)
	}
	if ack.Type != "subscribed" {
		t.Fatalf("ack type = %q", ack.Type)
	}

	got := map[string]bool{}
	for _, c := range ack.Channels {
		got[c] = true
	}
	if !got[SessionChannel("s1")] {
		t.Error("guest was not subscribed to its own session channel")
	}
	if !got[VenueMenuChannel("v1")] {
		t.Error("guest was not subscribed to the venue menu channel (FR-C4 stop-list push)")
	}
	if len(ack.Channels) != 2 {
		t.Errorf("channels = %v, want exactly the two a guest is entitled to", ack.Channels)
	}
}

// TestGuestCannotReachOtherChannels is the core authorisation property:
// channel names come from the verified token, so asking for someone else's
// gets you nothing.
func TestGuestCannotReachOtherChannels(t *testing.T) {
	hub := NewHub()
	h := NewHandler(hub,
		fakeStaffVerifier{},
		fakeGuestVerifier{identity: GuestIdentity{VenueID: "v1", TableID: "t1", GuestSessionID: "g1"}},
		fakeSessions{id: "s1"},
		nil)

	conn := dial(t, h, "/ws/guest")
	// Ask for another table's session and the venue's KDS stream.
	if err := conn.WriteJSON(subscribeFrame{
		Token:    "good",
		Channels: []string{SessionChannel("someone-elses"), VenueKDSChannel("v1")},
	}); err != nil {
		t.Fatalf("send subscribe: %v", err)
	}

	var ack ackFrame
	if err := readFrame(t, conn, &ack); err != nil {
		t.Fatalf("read ack: %v", err)
	}
	for _, c := range ack.Channels {
		if c == SessionChannel("someone-elses") {
			t.Fatal("a guest was subscribed to another table's session")
		}
		if c == VenueKDSChannel("v1") {
			t.Fatal("a guest was subscribed to the kitchen stream")
		}
	}
}

// TestGuestWithNoSessionYetStillConnects covers the guest who has scanned
// the QR code but not ordered: no session exists, which is normal, not an
// error.
func TestGuestWithNoSessionYetStillConnects(t *testing.T) {
	hub := NewHub()
	h := NewHandler(hub,
		fakeStaffVerifier{},
		fakeGuestVerifier{identity: GuestIdentity{VenueID: "v1", TableID: "t1", GuestSessionID: "g1"}},
		fakeSessions{id: ""},
		nil)

	conn := dial(t, h, "/ws/guest")
	if err := conn.WriteJSON(subscribeFrame{Token: "good"}); err != nil {
		t.Fatalf("send subscribe: %v", err)
	}

	var ack ackFrame
	if err := readFrame(t, conn, &ack); err != nil {
		t.Fatalf("read ack: %v", err)
	}
	if len(ack.Channels) != 1 || ack.Channels[0] != VenueMenuChannel("v1") {
		t.Errorf("channels = %v, want just the menu channel", ack.Channels)
	}
}

// TestGuestConnectsWhenSessionLookupFails: the order service being briefly
// unavailable should cost the guest their session channel, not the whole
// socket.
func TestGuestConnectsWhenSessionLookupFails(t *testing.T) {
	hub := NewHub()
	h := NewHandler(hub,
		fakeStaffVerifier{},
		fakeGuestVerifier{identity: GuestIdentity{VenueID: "v1", TableID: "t1", GuestSessionID: "g1"}},
		fakeSessions{err: errors.New("order service down")},
		nil)

	conn := dial(t, h, "/ws/guest")
	if err := conn.WriteJSON(subscribeFrame{Token: "good"}); err != nil {
		t.Fatalf("send subscribe: %v", err)
	}

	var ack ackFrame
	if err := readFrame(t, conn, &ack); err != nil {
		t.Fatalf("read ack: %v", err)
	}
	if ack.Type != "subscribed" {
		t.Errorf("connection was refused because a dependency was down: %+v", ack)
	}
}

func TestBadTokenIsRefusedWithAReason(t *testing.T) {
	hub := NewHub()
	h := NewHandler(hub,
		fakeStaffVerifier{err: errors.New("expired")},
		fakeGuestVerifier{err: errors.New("expired")},
		fakeSessions{},
		nil)

	for _, path := range []string{"/ws/guest", "/ws/staff"} {
		t.Run(path, func(t *testing.T) {
			conn := dial(t, h, path)
			if err := conn.WriteJSON(subscribeFrame{Token: "bad"}); err != nil {
				t.Fatalf("send subscribe: %v", err)
			}

			var frame errorFrame
			if err := readFrame(t, conn, &frame); err != nil {
				t.Fatalf("read error frame: %v", err)
			}
			// Naming the reason lets a reconnecting client re-authenticate
			// instead of retrying forever against a dead token.
			if frame.Code != "UNAUTHENTICATED" {
				t.Errorf("code = %q, want UNAUTHENTICATED", frame.Code)
			}
			if hub.Counts()[VenueKDSChannel("v1")] != 0 {
				t.Error("a rejected connection was subscribed anyway")
			}
		})
	}
}

func TestMalformedFirstFrameIsRefused(t *testing.T) {
	hub := NewHub()
	h := NewHandler(hub, fakeStaffVerifier{}, fakeGuestVerifier{}, fakeSessions{}, nil)

	conn := dial(t, h, "/ws/guest")
	if err := conn.WriteMessage(websocket.TextMessage, []byte("this is not json")); err != nil {
		t.Fatalf("send garbage: %v", err)
	}

	var frame errorFrame
	if err := readFrame(t, conn, &frame); err != nil {
		t.Fatalf("read error frame: %v", err)
	}
	if frame.Code != "VALIDATION_FAILED" {
		t.Errorf("code = %q, want VALIDATION_FAILED", frame.Code)
	}
}

// ---------------------------------------------------------------------
// Staff
// ---------------------------------------------------------------------

func TestStaffSubscribesToVenueStreams(t *testing.T) {
	hub := NewHub()
	h := NewHandler(hub,
		fakeStaffVerifier{identity: StaffIdentity{StaffID: "st1", VenueID: "v1", Role: "cook"}},
		fakeGuestVerifier{},
		fakeSessions{},
		nil)

	conn := dial(t, h, "/ws/staff")
	if err := conn.WriteJSON(subscribeFrame{Token: "good"}); err != nil {
		t.Fatalf("send subscribe: %v", err)
	}

	var ack ackFrame
	if err := readFrame(t, conn, &ack); err != nil {
		t.Fatalf("read ack: %v", err)
	}

	got := map[string]bool{}
	for _, c := range ack.Channels {
		got[c] = true
	}
	for _, want := range []string{VenueKDSChannel("v1"), VenueFloorChannel("v1"), VenueMenuChannel("v1")} {
		if !got[want] {
			t.Errorf("staff missing channel %q", want)
		}
	}
}

func TestStaffCanNarrowToRequestedChannels(t *testing.T) {
	hub := NewHub()
	h := NewHandler(hub,
		fakeStaffVerifier{identity: StaffIdentity{StaffID: "st1", VenueID: "v1", Role: "cook"}},
		fakeGuestVerifier{},
		fakeSessions{},
		nil)

	conn := dial(t, h, "/ws/staff")
	if err := conn.WriteJSON(subscribeFrame{
		Token:    "good",
		Channels: []string{VenueKDSChannel("v1"), VenueKDSChannel("other-venue")},
	}); err != nil {
		t.Fatalf("send subscribe: %v", err)
	}

	var ack ackFrame
	if err := readFrame(t, conn, &ack); err != nil {
		t.Fatalf("read ack: %v", err)
	}
	if len(ack.Channels) != 1 || ack.Channels[0] != VenueKDSChannel("v1") {
		t.Errorf("channels = %v, want only this venue's KDS stream", ack.Channels)
	}
}

// TestEventReachesASubscribedSocket is the end-to-end path a Kafka event
// takes once it is in the hub.
func TestEventReachesASubscribedSocket(t *testing.T) {
	hub := NewHub()
	h := NewHandler(hub,
		fakeStaffVerifier{identity: StaffIdentity{StaffID: "st1", VenueID: "v1", Role: "cook"}},
		fakeGuestVerifier{},
		fakeSessions{},
		nil)

	conn := dial(t, h, "/ws/staff")
	if err := conn.WriteJSON(subscribeFrame{Token: "good", Channels: []string{VenueKDSChannel("v1")}}); err != nil {
		t.Fatalf("send subscribe: %v", err)
	}
	var ack ackFrame
	if err := readFrame(t, conn, &ack); err != nil {
		t.Fatalf("read ack: %v", err)
	}

	hub.Broadcast(VenueKDSChannel("v1"), Event{
		EventID:   "e1",
		EventType: "order.placed",
		Payload:   json.RawMessage(`{"number":"A-014"}`),
	})

	var ev Event
	if err := readFrame(t, conn, &ev); err != nil {
		t.Fatalf("read event: %v", err)
	}
	if ev.EventID != "e1" || ev.EventType != "order.placed" {
		t.Errorf("event = %+v", ev)
	}
	if !strings.Contains(string(ev.Payload), "A-014") {
		t.Errorf("payload = %s", ev.Payload)
	}
}

func TestDisconnectUnsubscribes(t *testing.T) {
	hub := NewHub()
	h := NewHandler(hub,
		fakeStaffVerifier{identity: StaffIdentity{StaffID: "st1", VenueID: "v1"}},
		fakeGuestVerifier{},
		fakeSessions{},
		nil)

	conn := dial(t, h, "/ws/staff")
	if err := conn.WriteJSON(subscribeFrame{Token: "good"}); err != nil {
		t.Fatalf("send subscribe: %v", err)
	}
	var ack ackFrame
	if err := readFrame(t, conn, &ack); err != nil {
		t.Fatalf("read ack: %v", err)
	}
	if hub.Counts()[VenueKDSChannel("v1")] != 1 {
		t.Fatal("client was not registered")
	}

	_ = conn.Close()

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if len(hub.Counts()) == 0 {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Errorf("hub still holds channels after disconnect: %v", hub.Counts())
}

// ---------------------------------------------------------------------
// Origin policy
// ---------------------------------------------------------------------

func TestOriginChecker(t *testing.T) {
	check := originChecker([]string{"https://admin.example.com"})

	newReq := func(origin string) *http.Request {
		r := httptest.NewRequest(http.MethodGet, "http://gateway.internal/ws/staff", nil)
		r.Host = "gateway.internal"
		if origin != "" {
			r.Header.Set("Origin", origin)
		}
		return r
	}

	cases := []struct {
		name   string
		origin string
		want   bool
	}{
		{"no origin is a non-browser client", "", true},
		{"explicitly allowed", "https://admin.example.com", true},
		{"same origin over http", "http://gateway.internal", true},
		{"same origin over https", "https://gateway.internal", true},
		{"a hostile site", "https://evil.example.com", false},
		{"a lookalike host", "https://gateway.internal.evil.com", false},
		// Browsers do not enforce same-origin on WebSocket upgrades, so
		// without this check any page could open an authenticated-looking
		// socket to the gateway.
		{"null origin from a sandboxed frame", "null", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := check(newReq(tc.origin)); got != tc.want {
				t.Errorf("origin %q allowed = %v, want %v", tc.origin, got, tc.want)
			}
		})
	}
}

func TestIntersect(t *testing.T) {
	available := []string{"a", "b", "c"}

	if got := intersect(available, nil); len(got) != 3 {
		t.Errorf("no request should mean everything available, got %v", got)
	}
	if got := intersect(available, []string{"b"}); len(got) != 1 || got[0] != "b" {
		t.Errorf("intersect = %v, want [b]", got)
	}
	if got := intersect(available, []string{"z"}); len(got) != 0 {
		t.Errorf("intersect = %v, want empty for an unavailable channel", got)
	}
	if got := intersect(available, []string{"c", "a"}); len(got) != 2 {
		t.Errorf("intersect = %v, want both", got)
	}
}
