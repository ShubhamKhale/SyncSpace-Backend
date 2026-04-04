// main is the application entry point.
// Responsibilities:
//  1. Load configuration from the environment
//  2. Connect to the database
//  3. Wire dependencies (repo → service → controller)
//  4. Register routes
//  5. Start the HTTP server
package main

import (
	"context"
	"log"

	"github.com/gin-gonic/gin"

	"syncspace-backend/config"
	"syncspace-backend/constants"
	"syncspace-backend/controller"
	"syncspace-backend/pkg/db"
	"syncspace-backend/pkg/utils"
	"syncspace-backend/routes"
	"syncspace-backend/service"
	"syncspace-backend/service/database"
)

func main() {
	// ── 1. Configuration ──────────────────────────────────────────────────────
	cfg := config.Load()
	utils.Info(constants.LogTagMain, "configuration loaded")

	// ── 2. Database ───────────────────────────────────────────────────────────
	ctx := context.Background()
	if err := db.Connect(ctx, cfg.DBURL); err != nil {
		log.Fatalf("%s failed to connect to database: %v", constants.LogTagDB, err)
	}
	defer db.Close()
	utils.Info(constants.LogTagDB, "database connected")

	// ── 3. Dependency injection ───────────────────────────────────────────────
	boardRepo := database.NewBoardRepo(db.Pool)
	boardSvc := service.NewBoardService(boardRepo)

	healthCtrl := controller.NewHealthController()
	boardCtrl := controller.NewBoardController(boardSvc)

	// ── 4. Router ─────────────────────────────────────────────────────────────
	r := gin.Default()
	routes.SetupRoutes(r, healthCtrl, boardCtrl)

	// ── 5. Start server ───────────────────────────────────────────────────────
	addr := ":" + cfg.Port
	utils.Info(constants.LogTagMain, "starting server on "+addr)
	if err := r.Run(addr); err != nil {
		log.Fatalf("%s server failed: %v", constants.LogTagMain, err)
	}
}
