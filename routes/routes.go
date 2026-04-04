// Package routes wires HTTP routes to controller handlers.
// All path strings are sourced from the constants package to avoid duplication.
package routes

import (
	"github.com/gin-gonic/gin"

	"syncspace-backend/constants"
	"syncspace-backend/controller"
)

// SetupRoutes registers all application routes on the provided Gin engine.
// New route groups (e.g. /users, /diagrams) should be added here.
func SetupRoutes(r *gin.Engine, hc *controller.HealthController, bc *controller.BoardController) {
	// Liveness probe — no auth required
	r.GET(constants.URIHealth, hc.GetHealth)

	// Board resource
	r.POST(constants.URIBoards, bc.CreateBoard)
	r.GET(constants.URIBoards, bc.GetBoards)
}
