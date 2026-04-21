package ws

import (
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	"syncspace-backend/pkg/utils"
)

const (
	writeWait      = 10 * time.Second
	pongWait       = 60 * time.Second
	pingPeriod     = 50 * time.Second // must be < pongWait
	maxMessageSize = 64 * 1024        // 64 KB per frame
	sendBufSize    = 256              // outbound message buffer
)

// Upgrader is used by the controller to upgrade HTTP connections.
// CheckOrigin is permissive here — tighten for production with an allowlist.
var Upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

// Client wraps a single WebSocket connection.
// Each client is associated with exactly one authenticated user and can
// subscribe to any number of flow rooms during its lifetime.
type Client struct {
	hub    *Hub
	conn   *websocket.Conn
	userID string

	// send is the outbound message buffer; writePump drains it.
	send chan []byte

	// flows tracks which flow rooms this client has joined.
	flowsMu sync.RWMutex
	flows   map[string]struct{}
}

func newClient(hub *Hub, conn *websocket.Conn, userID string) *Client {
	return &Client{
		hub:    hub,
		conn:   conn,
		userID: userID,
		send:   make(chan []byte, sendBufSize),
		flows:  make(map[string]struct{}),
	}
}

// ── Flow subscription helpers ─────────────────────────────────────────────────

func (c *Client) joinFlow(flowID string) {
	c.flowsMu.Lock()
	c.flows[flowID] = struct{}{}
	c.flowsMu.Unlock()
}

func (c *Client) leaveFlow(flowID string) {
	c.flowsMu.Lock()
	delete(c.flows, flowID)
	c.flowsMu.Unlock()
}

// flowList returns a point-in-time snapshot of subscribed flow IDs.
func (c *Client) flowList() []string {
	c.flowsMu.RLock()
	defer c.flowsMu.RUnlock()
	list := make([]string, 0, len(c.flows))
	for id := range c.flows {
		list = append(list, id)
	}
	return list
}

// ── Pumps ─────────────────────────────────────────────────────────────────────

// readPump reads frames from the WebSocket and dispatches them to the hub.
// Must run in its own goroutine. Triggers unregister on any error.
func (c *Client) readPump() {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()
	}()

	c.conn.SetReadLimit(maxMessageSize)
	_ = c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		return c.conn.SetReadDeadline(time.Now().Add(pongWait))
	})

	for {
		_, raw, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err,
				websocket.CloseGoingAway,
				websocket.CloseAbnormalClosure,
			) {
				utils.Error("[WS]", "unexpected close user="+c.userID, err)
			}
			return
		}

		var msg Message
		if err := json.Unmarshal(raw, &msg); err != nil {
			c.enqueue(errMsg("invalid message format"))
			continue
		}
		c.hub.handleClientMessage(c, msg)
	}
}

// writePump drains the send buffer to the WebSocket and sends periodic pings.
// Must run in its own goroutine.
func (c *Client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case b, ok := <-c.send:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				// Hub closed the channel — send a close frame.
				_ = c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := c.conn.WriteMessage(websocket.TextMessage, b); err != nil {
				return
			}

		case <-ticker.C:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// enqueue encodes msg and places it in the send buffer.
// If the buffer is full the message is dropped — a slow client will eventually
// be disconnected by the write deadline.
func (c *Client) enqueue(msg Message) {
	b, err := json.Marshal(msg)
	if err != nil {
		return
	}
	select {
	case c.send <- b:
	default:
	}
}
