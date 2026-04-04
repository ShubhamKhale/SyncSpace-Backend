// Package controller contains HTTP handler types. Each controller receives its
// dependencies via constructor injection and exposes methods that satisfy the
// gin.HandlerFunc signature. Controllers must not contain business logic —
// they translate HTTP requests into service calls and HTTP responses.
package controller

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"syncspace-backend/service/data"
)

// HealthController handles liveness/readiness probe endpoints.
type HealthController struct{}

// NewHealthController creates a HealthController.
func NewHealthController() *HealthController {
	return &HealthController{}
}

// GetHealth godoc
// GET /health
// Returns a simple success payload to confirm the server is running.
func (h *HealthController) GetHealth(c *gin.Context) {
	c.JSON(http.StatusOK, data.OK(gin.H{"status": "ok"}))
}
