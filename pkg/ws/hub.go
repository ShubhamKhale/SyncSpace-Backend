package ws

import (
	"context"
	"encoding/json"
	"sync"

	"github.com/gorilla/websocket"
	"github.com/redis/go-redis/v9"

	"syncspace-backend/pkg/utils"
)

const (
	redisChanUser = "user:" // user:{userID}
	redisChanFlow = "flow:" // flow:{flowID}
)

// Hub is the central WebSocket router for a single server instance.
//
// Delivery strategy:
//  1. Local clients receive messages directly via their send channels.
//  2. Redis pub/sub replicates events to all other server instances.
//  3. Each Hub carries a unique serverID; messages it published are skipped
//     when received back from Redis to prevent duplicate local delivery.
type Hub struct {
	serverID string

	// mu protects userConns and flowConns — prefer RLock for reads.
	mu        sync.RWMutex
	userConns map[string]map[*Client]struct{} // userID → clients
	flowConns map[string]map[*Client]struct{} // flowID → clients

	register   chan *Client
	unregister chan *Client

	rdb *redis.Client
}

// NewHub constructs a Hub. Call hub.Run(ctx) to start it.
func NewHub(rdb *redis.Client, serverID string) *Hub {
	return &Hub{
		serverID:   serverID,
		userConns:  make(map[string]map[*Client]struct{}),
		flowConns:  make(map[string]map[*Client]struct{}),
		register:   make(chan *Client, 64),
		unregister: make(chan *Client, 64),
		rdb:        rdb,
	}
}

// Run starts the event loop and Redis subscriber goroutine.
// Blocks until ctx is cancelled.
func (h *Hub) Run(ctx context.Context) {
	go h.subscribeRedis(ctx)
	h.processEvents(ctx)
}

// Serve upgrades conn to a hub-managed WebSocket client for the given user.
// It registers the client, starts the write pump in a goroutine, then blocks
// in the read pump until the connection closes — matching Gin's handler lifetime.
func (h *Hub) Serve(conn *websocket.Conn, userID string) {
	c := newClient(h, conn, userID)
	h.register <- c
	go c.writePump()
	c.readPump() // blocks; triggers unregister on return
}

// ── Event loop ────────────────────────────────────────────────────────────────

func (h *Hub) processEvents(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return

		case c := <-h.register:
			h.mu.Lock()
			if h.userConns[c.userID] == nil {
				h.userConns[c.userID] = make(map[*Client]struct{})
			}
			h.userConns[c.userID][c] = struct{}{}
			h.mu.Unlock()
			utils.Info("[WS]", "connected user="+c.userID)

		case c := <-h.unregister:
			flows := c.flowList() // snapshot before cleanup

			h.mu.Lock()
			if clients, ok := h.userConns[c.userID]; ok {
				delete(clients, c)
				if len(clients) == 0 {
					delete(h.userConns, c.userID)
				}
			}
			for _, flowID := range flows {
				if clients, ok := h.flowConns[flowID]; ok {
					delete(clients, c)
					if len(clients) == 0 {
						delete(h.flowConns, flowID)
					}
				}
			}
			h.mu.Unlock()
			close(c.send)
			utils.Info("[WS]", "disconnected user="+c.userID)

			// Broadcast presence.leave for every flow this client had joined.
			for _, flowID := range flows {
				h.publishFlow(ctx, flowID, Message{
					Type:    TypePresenceLeave,
					Payload: mustMarshal(PresencePayload{FlowID: flowID, UserID: c.userID}),
				})
			}
		}
	}
}

// ── Client message dispatcher ─────────────────────────────────────────────────

// handleClientMessage routes inbound WebSocket frames from a client.
func (h *Hub) handleClientMessage(c *Client, msg Message) {
	switch msg.Type {

	case TypeFlowJoin:
		var p FlowActionPayload
		if err := json.Unmarshal(msg.Payload, &p); err != nil || p.FlowID == "" {
			c.enqueue(errMsg("flow.join requires {\"flow_id\":\"...\"}"))
			return
		}
		c.joinFlow(p.FlowID)

		h.mu.Lock()
		if h.flowConns[p.FlowID] == nil {
			h.flowConns[p.FlowID] = make(map[*Client]struct{})
		}
		h.flowConns[p.FlowID][c] = struct{}{}
		h.mu.Unlock()

		// Send the joining client a snapshot of locally-known members.
		localUsers := h.flowUserIDs(p.FlowID, c.userID)
		c.enqueue(Message{
			Type:    TypePresenceList,
			Payload: mustMarshal(PresenceListPayload{FlowID: p.FlowID, UserIDs: localUsers}),
		})

		// Announce arrival to all flow subscribers (including other instances).
		h.publishFlow(context.Background(), p.FlowID, Message{
			Type:    TypePresenceJoin,
			Payload: mustMarshal(PresencePayload{FlowID: p.FlowID, UserID: c.userID}),
		})

	case TypeFlowLeave:
		var p FlowActionPayload
		if err := json.Unmarshal(msg.Payload, &p); err != nil || p.FlowID == "" {
			return
		}
		c.leaveFlow(p.FlowID)

		h.mu.Lock()
		if clients, ok := h.flowConns[p.FlowID]; ok {
			delete(clients, c)
			if len(clients) == 0 {
				delete(h.flowConns, p.FlowID)
			}
		}
		h.mu.Unlock()

		h.publishFlow(context.Background(), p.FlowID, Message{
			Type:    TypePresenceLeave,
			Payload: mustMarshal(PresencePayload{FlowID: p.FlowID, UserID: c.userID}),
		})

	case TypeCursorUpdate:
		var p CursorPayload
		if err := json.Unmarshal(msg.Payload, &p); err != nil || p.FlowID == "" {
			return
		}
		p.UserID = c.userID // always use the authenticated user ID
		h.publishFlow(context.Background(), p.FlowID, Message{
			Type:    TypeCursorMove,
			Payload: mustMarshal(p),
		})

	case TypePresentationStart:
		var p PresentationStartPayload
		if err := json.Unmarshal(msg.Payload, &p); err != nil || p.FlowID == "" {
			c.enqueue(errMsg("presentation.start requires {\"flow_id\":\"...\",\"presenter_name\":\"...\"}"))
			return
		}
		h.publishChannel(context.Background(), redisChanFlow+p.FlowID, c.userID, Message{
			Type: TypePresentationStarted,
			Payload: mustMarshal(PresentationStartedPayload{
				FlowID:        p.FlowID,
				PresenterID:   c.userID,
				PresenterName: p.PresenterName,
			}),
		})

	case TypePresentationStop:
		var p FlowActionPayload
		if err := json.Unmarshal(msg.Payload, &p); err != nil || p.FlowID == "" {
			return
		}
		h.publishChannel(context.Background(), redisChanFlow+p.FlowID, c.userID, Message{
			Type:    TypePresentationStopped,
			Payload: mustMarshal(PresentationStoppedPayload{FlowID: p.FlowID}),
		})

	case TypePresentationSlide:
		var p PresentationSlidePayload
		if err := json.Unmarshal(msg.Payload, &p); err != nil || p.FlowID == "" {
			return
		}
		h.publishChannel(context.Background(), redisChanFlow+p.FlowID, c.userID, Message{
			Type: TypePresentationSlide,
			Payload: mustMarshal(PresentationSlideOutPayload{
				FlowID:      p.FlowID,
				NodeID:      p.NodeID,
				SlideIndex:  p.SlideIndex,
				PresenterID: c.userID,
			}),
		})
	}
}

// ── Local delivery ────────────────────────────────────────────────────────────

// deliverToUser sends msg to all local connections of userID.
func (h *Hub) deliverToUser(userID string, msg Message) {
	h.mu.RLock()
	clients := h.userConns[userID]
	h.mu.RUnlock()

	for c := range clients {
		c.enqueue(msg)
	}
}

// deliverToFlow sends msg to all local clients subscribed to flowID,
// optionally excluding one user (e.g. the sender).
func (h *Hub) deliverToFlow(flowID, excludeUserID string, msg Message) {
	h.mu.RLock()
	clients := h.flowConns[flowID]
	h.mu.RUnlock()

	for c := range clients {
		if c.userID != excludeUserID {
			c.enqueue(msg)
		}
	}
}

// flowUserIDs returns the user IDs of local clients in a flow, excluding one.
func (h *Hub) flowUserIDs(flowID, excludeUserID string) []string {
	h.mu.RLock()
	clients := h.flowConns[flowID]
	h.mu.RUnlock()

	seen := make(map[string]struct{})
	for c := range clients {
		if c.userID != excludeUserID {
			seen[c.userID] = struct{}{}
		}
	}
	ids := make([]string, 0, len(seen))
	for uid := range seen {
		ids = append(ids, uid)
	}
	return ids
}

// ── Redis pub/sub ─────────────────────────────────────────────────────────────

// PublishToUser publishes msg to all instances for a specific user.
// Call this from services (e.g. notification service) to push real-time updates.
func (h *Hub) PublishToUser(ctx context.Context, userID string, msg Message) {
	h.publishChannel(ctx, redisChanUser+userID, "", msg)
}

// PublishToFlowExcluding broadcasts msg to all flow participants except senderID.
// Safe to call when hub is nil (no-op). Use after a successful DB write to avoid
// broadcasting on validation or auth failures.
func (h *Hub) PublishToFlowExcluding(ctx context.Context, flowID, senderID string, msg Message) {
	h.publishChannel(ctx, redisChanFlow+flowID, senderID, msg)
}

// publishFlow publishes a flow-scoped message to all instances.
func (h *Hub) publishFlow(ctx context.Context, flowID string, msg Message) {
	h.publishChannel(ctx, redisChanFlow+flowID, "", msg)
}

func (h *Hub) publishChannel(ctx context.Context, channel, excludeUserID string, msg Message) {
	env := RedisEnvelope{
		SourceID:      h.serverID,
		Channel:       channel,
		Msg:           msg,
		ExcludeUserID: excludeUserID,
	}
	b, err := json.Marshal(env)
	if err != nil {
		utils.Error("[WS]", "failed to marshal redis envelope", err)
		return
	}

	// Deliver to local clients immediately (before Redis round-trip).
	h.deliverLocal(env)

	// Publish to Redis for other instances.
	if err := h.rdb.Publish(ctx, channel, b).Err(); err != nil {
		utils.Error("[WS]", "redis publish failed channel="+channel, err)
	}
}

// subscribeRedis listens for messages published by other server instances
// and relays them to locally-connected clients.
func (h *Hub) subscribeRedis(ctx context.Context) {
	pubsub := h.rdb.PSubscribe(ctx, redisChanUser+"*", redisChanFlow+"*")
	defer pubsub.Close()

	ch := pubsub.Channel()
	for {
		select {
		case <-ctx.Done():
			return
		case redisMsg, ok := <-ch:
			if !ok {
				return
			}
			var env RedisEnvelope
			if err := json.Unmarshal([]byte(redisMsg.Payload), &env); err != nil {
				utils.Error("[WS]", "failed to unmarshal redis message", err)
				continue
			}
			// Skip messages this instance published — already delivered locally.
			if env.SourceID == h.serverID {
				continue
			}
			h.deliverLocal(env)
		}
	}
}

// deliverLocal routes an envelope to the appropriate local clients based on channel prefix.
func (h *Hub) deliverLocal(env RedisEnvelope) {
	switch {
	case len(env.Channel) > len(redisChanUser) && env.Channel[:len(redisChanUser)] == redisChanUser:
		userID := env.Channel[len(redisChanUser):]
		h.deliverToUser(userID, env.Msg)

	case len(env.Channel) > len(redisChanFlow) && env.Channel[:len(redisChanFlow)] == redisChanFlow:
		flowID := env.Channel[len(redisChanFlow):]
		h.deliverToFlow(flowID, env.ExcludeUserID, env.Msg)
	}
}
