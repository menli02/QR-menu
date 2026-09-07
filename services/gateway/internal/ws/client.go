package ws

import (
	"time"

	"github.com/gorilla/websocket"
	"github.com/zeromicro/go-zero/core/logx"
)

// Timings from docs/TZ.md §7.3: "Heartbeat: ping every 25 s, drop after 2
// missed pongs."
const (
	pingInterval = 25 * time.Second
	pongTimeout  = 2*pingInterval + 5*time.Second // two missed pongs, plus slack for a slow network
	writeTimeout = 10 * time.Second

	// sendBuffer is how far behind a client may fall before its messages
	// start being dropped. A KDS screen sees a handful of events a minute;
	// 64 absorbs a burst (a party ordering at once) without letting a dead
	// tab hold unbounded memory.
	sendBuffer = 64

	// maxIncomingMessage bounds the auth frame. Clients send exactly one
	// message — the subscribe frame — and anything larger is a client bug
	// or an attempt to exhaust memory.
	maxIncomingMessage = 4 * 1024
)

// Client is one WebSocket connection.
//
// All writes go through the single writePump goroutine. A gorilla
// connection does not support concurrent writers, and the heartbeat writes
// too, so funnelling everything through one goroutine is what makes that
// safe — rather than a mutex around every write.
type Client struct {
	hub  *Hub
	conn *websocket.Conn
	send chan []byte

	// Description of who this is, for logs. Never sent to the peer.
	label string
}

func newClient(hub *Hub, conn *websocket.Conn, label string) *Client {
	return &Client{
		hub:   hub,
		conn:  conn,
		send:  make(chan []byte, sendBuffer),
		label: label,
	}
}

// run drives the connection until it closes. It blocks, so the caller
// (the HTTP handler) stays alive for the life of the socket, which is what
// keeps the upgraded connection from being torn down.
func (c *Client) run() {
	done := make(chan struct{})
	go func() {
		c.writePump()
		close(done)
	}()

	c.readPump()
	<-done
}

// readPump consumes from the peer.
//
// Clients have nothing to say after the subscribe frame — every action is
// a REST call, deliberately, so that writes go through the same
// validation, authorisation and idempotency as any other request. So this
// exists to notice the connection dying and to service pongs, not to
// dispatch commands. Anything a client does send is discarded.
func (c *Client) readPump() {
	defer func() {
		c.hub.Unsubscribe(c)
		_ = c.conn.Close()
		close(c.send)
	}()

	c.conn.SetReadLimit(maxIncomingMessage)
	_ = c.conn.SetReadDeadline(time.Now().Add(pongTimeout))
	c.conn.SetPongHandler(func(string) error {
		return c.conn.SetReadDeadline(time.Now().Add(pongTimeout))
	})

	for {
		if _, _, err := c.conn.ReadMessage(); err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
				logx.Errorf("ws: %s read error: %v", c.label, err)
			}
			return
		}
		// Deliberately ignored — see the doc comment.
		_ = c.conn.SetReadDeadline(time.Now().Add(pongTimeout))
	}
}

// writePump is the only goroutine that writes to the connection.
func (c *Client) writePump() {
	ticker := time.NewTicker(pingInterval)
	defer func() {
		ticker.Stop()
		_ = c.conn.Close()
	}()

	for {
		select {
		case msg, ok := <-c.send:
			if !ok {
				// readPump closed the channel: the connection is going
				// away. Tell the peer politely so it can reconnect
				// immediately rather than waiting for a timeout.
				_ = c.conn.SetWriteDeadline(time.Now().Add(writeTimeout))
				_ = c.conn.WriteMessage(websocket.CloseMessage,
					websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
				return
			}
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeTimeout))
			if err := c.conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				logx.Errorf("ws: %s write error: %v", c.label, err)
				return
			}

		case <-ticker.C:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeTimeout))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
