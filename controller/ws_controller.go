package controller

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"

	"syncspace-backend/pkg/utils"
	"syncspace-backend/pkg/ws"
	"syncspace-backend/service/data"
)

// WsController upgrades HTTP connections to WebSocket and registers clients
// with the hub. Authentication is via JWT in the ?token= query parameter
// (browsers cannot set Authorization headers during WebSocket handshakes).
type WsController struct {
	hub       *ws.Hub
	jwtSecret string
}

// NewWsController creates a WsController backed by the given hub.
func NewWsController(hub *ws.Hub, jwtSecret string) *WsController {
	return &WsController{hub: hub, jwtSecret: jwtSecret}
}

// ServeWs handles GET /ws.
// Returns 503 when Redis is not configured (hub is nil).
//
// Query parameters:
//
//	?token=<JWT>   — required; the same JWT issued by POST /auth/signin
//
// On successful upgrade the connection stays open until the client disconnects
// or the server shuts down. All subsequent communication is via JSON frames.
//
// Client → server frames:
//
//	{ "type": "flow.join",     "payload": { "flow_id": "..." } }
//	{ "type": "flow.leave",    "payload": { "flow_id": "..." } }
//	{ "type": "cursor.update", "payload": { "flow_id": "...", "x": 1.0, "y": 2.0 } }
//
// Server → client frames:
//
//	{ "type": "notification",   "payload": { ... } }
//	{ "type": "presence.join",  "payload": { "flow_id": "...", "user_id": "..." } }
//	{ "type": "presence.leave", "payload": { "flow_id": "...", "user_id": "..." } }
//	{ "type": "presence.list",  "payload": { "flow_id": "...", "user_ids": [...] } }
//	{ "type": "cursor.move",    "payload": { "flow_id": "...", "user_id": "...", "x": 1.0, "y": 2.0 } }
func (wc *WsController) ServeWs(c *gin.Context) {
	if wc.hub == nil {
		c.JSON(http.StatusServiceUnavailable, data.Fail("WebSocket is not available (REDIS_URL not configured)"))
		return
	}

	tokenStr := c.Query("token")
	if tokenStr == "" {
		c.JSON(http.StatusUnauthorized, data.Fail("token query parameter is required"))
		return
	}

	userID, err := wc.parseToken(tokenStr)
	if err != nil {
		utils.Error("[WS]", "invalid token", err)
		c.JSON(http.StatusUnauthorized, data.Fail("invalid or expired token"))
		return
	}

	conn, err := ws.Upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		utils.Error("[WS]", "upgrade failed user="+userID, err)
		// Upgrader already wrote the HTTP error response.
		return
	}

	// Serve blocks until the connection closes (read pump exit).
	// The hub registers and deregisters the client transparently.
	wc.hub.Serve(conn, userID)
}

// parseToken validates the JWT and extracts the user_id claim.
func (wc *WsController) parseToken(tokenStr string) (string, error) {
	token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, jwt.ErrSignatureInvalid
		}
		return []byte(wc.jwtSecret), nil
	})
	if err != nil || !token.Valid {
		return "", jwt.ErrSignatureInvalid
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return "", jwt.ErrInvalidType
	}

	userID, _ := claims["sub"].(string)
	if userID == "" {
		return "", jwt.ErrInvalidType
	}
	return userID, nil
}
