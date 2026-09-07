package ws

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
	"github.com/zeromicro/go-zero/core/logx"
)

// authTimeout bounds how long a connection may sit unauthenticated. A
// socket that upgrades and then says nothing costs a file descriptor and
// a goroutine; without this, opening thousands of them is a free denial
// of service.
const authTimeout = 10 * time.Second

// subscribeFrame is the single message a client sends, immediately after
// the upgrade.
//
// The token travels here rather than in a query string because §8.1 says
// so, and the reason is worth keeping in view: query strings end up in
// proxy access logs, browser history and Referer headers, and a guest JWT
// is a bearer credential for someone's table.
type subscribeFrame struct {
	Token string `json:"token"`
	// Channels the client wants. Empty means "everything I'm entitled
	// to", which is what both real clients want and saves them having to
	// know the channel naming scheme.
	Channels []string `json:"channels,omitempty"`
}

// ackFrame confirms what the server actually subscribed the client to,
// which will differ from what it asked for if it requested a channel it
// isn't entitled to. Telling it explicitly beats leaving it to wonder why
// nothing arrives.
type ackFrame struct {
	Type     string   `json:"type"`
	Channels []string `json:"channels"`
}

type errorFrame struct {
	Type    string `json:"type"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

// StaffVerifier and GuestVerifier are the token-checking half of the auth
// middlewares. Narrow interfaces rather than the concrete types so this
// package doesn't depend on the middleware's construction, and so tests
// can supply their own.
type StaffVerifier interface {
	Verify(ctx context.Context, token string) (StaffIdentity, error)
}

type GuestVerifier interface {
	Verify(ctx context.Context, token string) (GuestIdentity, error)
}

// StaffIdentity and GuestIdentity mirror the middleware's claim structs.
// Redeclared here to keep the dependency one-way (middleware knows nothing
// about ws); the adapters in svc bridge the two.
type StaffIdentity struct {
	StaffID string
	VenueID string
	Role    string
}

type GuestIdentity struct {
	VenueID        string
	TableID        string
	GuestSessionID string
}

// SessionResolver answers "which table session is this guest's?", so a
// guest can be subscribed to its own session channel and no other.
type SessionResolver interface {
	CurrentSessionID(ctx context.Context, venueID, tableID string) (string, error)
}

type Handler struct {
	hub      *Hub
	staff    StaffVerifier
	guest    GuestVerifier
	sessions SessionResolver
	upgrader websocket.Upgrader
}

func NewHandler(hub *Hub, staff StaffVerifier, guest GuestVerifier, sessions SessionResolver, allowedOrigins []string) *Handler {
	return &Handler{
		hub:      hub,
		staff:    staff,
		guest:    guest,
		sessions: sessions,
		upgrader: websocket.Upgrader{
			HandshakeTimeout: authTimeout,
			ReadBufferSize:   1024,
			WriteBufferSize:  4096,
			CheckOrigin:      originChecker(allowedOrigins),
		},
	}
}

// originChecker enforces the WebSocket origin policy.
//
// This is not decoration. Browsers do not apply the same-origin policy to
// WebSocket handshakes and do send cookies with them, so a socket endpoint
// without an origin check is the classic cross-site hijacking hole. The
// token-in-first-frame design already means a hostile page cannot
// authenticate (it has no token), but the check costs nothing and closes
// the door on the resource-exhaustion half of the problem too.
//
// An empty allow-list rejects every cross-origin handshake and permits
// same-origin ones, which is the right default for a service whose
// frontends are served from the same host.
func originChecker(allowed []string) func(*http.Request) bool {
	set := make(map[string]struct{}, len(allowed))
	for _, o := range allowed {
		set[o] = struct{}{}
	}
	return func(r *http.Request) bool {
		origin := r.Header.Get("Origin")
		if origin == "" {
			// Not a browser — a native app or a test client. Those cannot
			// be tricked into a cross-site handshake, so there is nothing
			// for this check to protect against.
			return true
		}
		if _, ok := set[origin]; ok {
			return true
		}
		if origin == "http://"+r.Host || origin == "https://"+r.Host {
			return true
		}
		logx.Errorf("ws: rejected handshake from origin %q", origin)
		return false
	}
}

// ServeGuest handles GET /ws/guest.
func (h *Handler) ServeGuest(w http.ResponseWriter, r *http.Request) {
	h.serve(w, r, h.authoriseGuest)
}

// ServeStaff handles GET /ws/staff.
func (h *Handler) ServeStaff(w http.ResponseWriter, r *http.Request) {
	h.serve(w, r, h.authoriseStaff)
}

// authoriser turns a subscribe frame into the channels a client may join,
// or an error frame explaining why not.
type authoriser func(ctx context.Context, frame subscribeFrame) (channels []string, label string, err *errorFrame)

func (h *Handler) serve(w http.ResponseWriter, r *http.Request, authorise authoriser) {
	conn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		// Upgrade has already written a response.
		logx.Errorf("ws: upgrade failed: %v", err)
		return
	}

	frame, err := readSubscribeFrame(conn)
	if err != nil {
		writeErrorAndClose(conn, &errorFrame{Type: "error", Code: "VALIDATION_FAILED", Message: "expected a subscribe frame"})
		return
	}

	channels, label, authErr := authorise(r.Context(), frame)
	if authErr != nil {
		writeErrorAndClose(conn, authErr)
		return
	}

	client := newClient(h.hub, conn, label)
	for _, ch := range channels {
		h.hub.Subscribe(client, ch)
	}

	if err := writeJSON(conn, ackFrame{Type: "subscribed", Channels: channels}); err != nil {
		h.hub.Unsubscribe(client)
		_ = conn.Close()
		return
	}

	logx.Infof("ws: %s subscribed to %v", label, channels)
	client.run()
	logx.Infof("ws: %s disconnected", label)
}

// authoriseGuest is the narrowest authorisation in the system, and
// intentionally so: a guest may watch its *own* table session and its
// venue's menu, and nothing else. Both channel names are derived from the
// verified token, never from the request, so there is no id for a guest to
// substitute in order to watch another table.
func (h *Handler) authoriseGuest(ctx context.Context, frame subscribeFrame) ([]string, string, *errorFrame) {
	identity, err := h.guest.Verify(ctx, frame.Token)
	if err != nil {
		return nil, "", &errorFrame{Type: "error", Code: "UNAUTHENTICATED", Message: "invalid or expired guest token"}
	}

	// The menu channel carries stop-list changes (FR-C4's 5-second
	// promise); every guest at the venue needs it.
	channels := []string{VenueMenuChannel(identity.VenueID)}

	// The session channel carries this party's order updates. A guest who
	// has not ordered yet has no session, which is not an error — they
	// simply get the menu channel until they do, and reconnect after
	// their first order.
	sessionID, err := h.sessions.CurrentSessionID(ctx, identity.VenueID, identity.TableID)
	if err != nil {
		logx.Errorf("ws: cannot resolve session for table %s: %v", identity.TableID, err)
	} else if sessionID != "" {
		channels = append(channels, SessionChannel(sessionID))
	}

	return channels, "guest " + identity.GuestSessionID, nil
}

// authoriseStaff subscribes a staff client to its venue's kitchen and
// floor streams. The venue comes from the token, so a staff member of one
// venue cannot watch another's.
//
// No role check: every staff role has a legitimate reason to watch. A cook
// needs tickets, a waiter needs service requests, and a manager wants
// both. Actions remain REST calls, where the role checks live.
func (h *Handler) authoriseStaff(ctx context.Context, frame subscribeFrame) ([]string, string, *errorFrame) {
	identity, err := h.staff.Verify(ctx, frame.Token)
	if err != nil {
		return nil, "", &errorFrame{Type: "error", Code: "UNAUTHENTICATED", Message: "invalid or expired staff token"}
	}

	available := []string{
		VenueKDSChannel(identity.VenueID),
		VenueFloorChannel(identity.VenueID),
		VenueMenuChannel(identity.VenueID),
	}
	return intersect(available, frame.Channels), "staff " + identity.StaffID, nil
}

// intersect narrows `available` to what the client asked for. An empty
// request means everything available — the common case, and it spares
// clients from hard-coding the channel naming scheme. A requested channel
// that isn't available is silently dropped rather than refused: the ack
// frame reports what was actually granted, so the client can tell.
func intersect(available, requested []string) []string {
	if len(requested) == 0 {
		return available
	}
	want := make(map[string]struct{}, len(requested))
	for _, r := range requested {
		want[r] = struct{}{}
	}
	out := make([]string, 0, len(available))
	for _, a := range available {
		if _, ok := want[a]; ok {
			out = append(out, a)
		}
	}
	return out
}

func readSubscribeFrame(conn *websocket.Conn) (subscribeFrame, error) {
	conn.SetReadLimit(maxIncomingMessage)
	if err := conn.SetReadDeadline(time.Now().Add(authTimeout)); err != nil {
		return subscribeFrame{}, err
	}
	var frame subscribeFrame
	if err := conn.ReadJSON(&frame); err != nil {
		return subscribeFrame{}, err
	}
	return frame, nil
}

func writeJSON(conn *websocket.Conn, v any) error {
	if err := conn.SetWriteDeadline(time.Now().Add(writeTimeout)); err != nil {
		return err
	}
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return conn.WriteMessage(websocket.TextMessage, data)
}

// writeErrorAndClose tells the client why it was refused before hanging
// up. A bare close leaves a reconnecting client retrying forever against
// an expired token; naming the reason lets it go and re-authenticate.
func writeErrorAndClose(conn *websocket.Conn, frame *errorFrame) {
	_ = writeJSON(conn, frame)
	_ = conn.SetWriteDeadline(time.Now().Add(writeTimeout))
	_ = conn.WriteMessage(websocket.CloseMessage,
		websocket.FormatCloseMessage(websocket.ClosePolicyViolation, frame.Code))
	_ = conn.Close()
}
