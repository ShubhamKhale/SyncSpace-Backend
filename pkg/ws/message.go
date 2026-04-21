// Package ws implements the WebSocket hub, client lifecycle, and message types.
package ws

import "encoding/json"

// ── Message type constants ────────────────────────────────────────────────────

const (
	// Server → client
	TypeNotification = "notification"  // new notification pushed to a user
	TypePresenceJoin = "presence.join"  // a user joined a flow room
	TypePresenceLeave = "presence.leave" // a user left a flow room
	TypePresenceList  = "presence.list"  // snapshot of users in a room (sent on join)
	TypeCursorMove    = "cursor.move"    // another user's cursor position
	TypeError         = "error"          // error acknowledgement

	// Client → server
	TypeFlowJoin     = "flow.join"      // subscribe to a flow room
	TypeFlowLeave    = "flow.leave"     // unsubscribe from a flow room
	TypeCursorUpdate = "cursor.update"  // my cursor moved (server rebroadcasts as cursor.move)
)

// ── Wire envelopes ────────────────────────────────────────────────────────────

// Message is the JSON envelope for every WebSocket frame.
//
//	{ "type": "cursor.move", "payload": { ... } }
type Message struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload,omitempty"`
}

// RedisEnvelope is published to Redis pub/sub channels so other server
// instances can relay the message to their locally-connected clients.
// SourceID prevents the publishing instance from re-delivering its own messages.
type RedisEnvelope struct {
	SourceID string  `json:"source_id"` // server instance that published
	Channel  string  `json:"channel"`   // e.g. "user:abc" or "flow:xyz"
	Msg      Message `json:"msg"`
}

// ── Payload structs ───────────────────────────────────────────────────────────

// CursorPayload is the payload for TypeCursorUpdate (inbound) and TypeCursorMove (outbound).
type CursorPayload struct {
	FlowID string  `json:"flow_id"`
	UserID string  `json:"user_id"`
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
}

// PresencePayload is the payload for TypePresenceJoin / TypePresenceLeave.
type PresencePayload struct {
	FlowID string `json:"flow_id"`
	UserID string `json:"user_id"`
}

// PresenceListPayload is sent to a client immediately after it joins a flow.
// It lists the user IDs of all other locally-connected members of that flow.
type PresenceListPayload struct {
	FlowID  string   `json:"flow_id"`
	UserIDs []string `json:"user_ids"`
}

// FlowActionPayload is the payload for TypeFlowJoin / TypeFlowLeave (client → server).
type FlowActionPayload struct {
	FlowID string `json:"flow_id"`
}

// ── Helpers ───────────────────────────────────────────────────────────────────

// mustMarshal JSON-encodes v; panics only on programming errors (unserializable types).
func mustMarshal(v any) json.RawMessage {
	b, err := json.Marshal(v)
	if err != nil {
		panic("ws: mustMarshal: " + err.Error())
	}
	return b
}

func errMsg(text string) Message {
	return Message{Type: TypeError, Payload: mustMarshal(text)}
}
