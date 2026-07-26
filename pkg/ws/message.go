// Package ws implements the WebSocket hub, client lifecycle, and message types.
package ws

import "encoding/json"

// ── Message type constants ────────────────────────────────────────────────────

const (
	// Server → client
	TypeNotification         = "notification"          // new notification pushed to a user
	TypePresenceJoin         = "presence.join"         // a user joined a flow room
	TypePresenceLeave        = "presence.leave"        // a user left a flow room
	TypePresenceList         = "presence.list"         // snapshot of users in a room (sent on join)
	TypeCursorMove           = "cursor.move"           // another user's cursor position
	TypeDiagramUpdated       = "diagram_updated"       // another user saved the diagram
	TypePresentationStarted  = "presentation.started"  // presentation began
	TypePresentationStopped  = "presentation.stopped"  // presentation ended
	TypeError                = "error"                 // error acknowledgement

	// Client → server
	TypeFlowJoin          = "flow.join"            // subscribe to a flow room
	TypeFlowLeave         = "flow.leave"           // unsubscribe from a flow room
	TypeCursorUpdate      = "cursor.update"        // my cursor moved (server rebroadcasts as cursor.move)
	TypePresentationStart = "presentation.start"   // begin presenting to room
	TypePresentationStop  = "presentation.stop"    // end presentation
	TypePresentationSlide = "presentation.slide"   // navigate to a slide
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
	SourceID      string  `json:"source_id"`                 // server instance that published
	Channel       string  `json:"channel"`                   // e.g. "user:abc" or "flow:xyz"
	Msg           Message `json:"msg"`
	ExcludeUserID string  `json:"exclude_user_id,omitempty"` // skip this user on delivery
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

// DiagramUpdatedPayload is broadcast to all other participants when a diagram is saved.
type DiagramUpdatedPayload struct {
	Nodes json.RawMessage `json:"nodes"`
	Edges json.RawMessage `json:"edges"`
}

// PresentationStartPayload is sent by the presenter to begin a presentation (client → server).
type PresentationStartPayload struct {
	FlowID        string `json:"flow_id"`
	PresenterName string `json:"presenter_name"`
}

// PresentationStartedPayload is broadcast to all other room members when a presentation begins.
type PresentationStartedPayload struct {
	FlowID        string `json:"flow_id"`
	PresenterID   string `json:"presenter_id"`
	PresenterName string `json:"presenter_name"`
}

// PresentationStoppedPayload is broadcast when the presenter ends the session.
type PresentationStoppedPayload struct {
	FlowID string `json:"flow_id"`
}

// PresentationSlidePayload is sent by the presenter to advance a slide (client → server).
type PresentationSlidePayload struct {
	FlowID     string `json:"flow_id"`
	NodeID     string `json:"node_id"`
	SlideIndex int    `json:"slide_index"`
}

// PresentationSlideOutPayload is broadcast to all other room members on slide navigation.
type PresentationSlideOutPayload struct {
	FlowID      string `json:"flow_id"`
	NodeID      string `json:"node_id"`
	SlideIndex  int    `json:"slide_index"`
	PresenterID string `json:"presenter_id"`
}

// ── Helpers ───────────────────────────────────────────────────────────────────

// MustMarshal JSON-encodes v; panics only on programming errors (unserializable types).
func MustMarshal(v any) json.RawMessage {
	b, err := json.Marshal(v)
	if err != nil {
		panic("ws: MustMarshal: " + err.Error())
	}
	return b
}

func mustMarshal(v any) json.RawMessage { return MustMarshal(v) }

func errMsg(text string) Message {
	return Message{Type: TypeError, Payload: mustMarshal(text)}
}
