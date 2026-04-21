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
	pkgredis "syncspace-backend/pkg/redis"
	"syncspace-backend/pkg/utils"
	pkgws "syncspace-backend/pkg/ws"
	"syncspace-backend/routes"
	"syncspace-backend/service"
	"syncspace-backend/service/database"
	"syncspace-backend/service/session"
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

	// ── 2b. Redis (optional — WebSocket disabled when REDIS_URL is unset) ─────
	var hub *pkgws.Hub
	if cfg.RedisURL != "" {
		if err := pkgredis.Connect(ctx, cfg.RedisURL); err != nil {
			log.Fatalf("[REDIS] failed to connect: %v", err)
		}
		defer pkgredis.Close()
		utils.Info("[REDIS]", "connected")

		serverID := utils.NewUUID()
		hub = pkgws.NewHub(pkgredis.Client, serverID)
		go hub.Run(ctx)
		utils.Info("[WS]", "hub started server_id="+serverID)
	} else {
		utils.Info("[WS]", "REDIS_URL not set — WebSocket hub disabled")
	}

	// ── 3. Dependency injection ───────────────────────────────────────────────
	// Session store — in-memory AES key registry (no DB)
	store := session.NewStore()

	// Repos
	userRepo      := database.NewUserRepo(db.Pool)
	boardRepo     := database.NewBoardRepo(db.Pool)
	orgRepo       := database.NewOrgRepo(db.Pool)
	taskRepo      := database.NewTaskRepo(db.Pool)
	dashboardRepo  := database.NewDashboardRepo(db.Pool)
	activityRepo   := database.NewActivityRepo(db.Pool)
	analyticsRepo     := database.NewAnalyticsRepo(db.Pool)
	notificationRepo    := database.NewNotificationRepo(db.Pool)
	linkedResourceRepo  := database.NewLinkedResourceRepo(db.Pool)
	flowRepo            := database.NewFlowRepo(db.Pool)
	flowVoteRepo        := database.NewFlowVoteRepo(db.Pool)

	// Activity logger — injected into services that perform writes
	activityLogger := service.NewActivityLogger(activityRepo)

	// Services
	authSvc      := service.NewAuthService(userRepo, store, cfg.JWTSecret)
	userSvc      := service.NewUserService(userRepo)
	boardSvc     := service.NewBoardService(boardRepo, activityLogger)
	orgSvc       := service.NewOrgService(orgRepo, userRepo)
	taskSvc      := service.NewTaskService(taskRepo, activityLogger)
	dashboardSvc  := service.NewDashboardService(dashboardRepo, boardRepo, taskRepo)
	activitySvc   := service.NewActivityService(activityRepo)
	analyticsSvc     := service.NewAnalyticsService(analyticsRepo)
	notificationSvc      := service.NewNotificationService(notificationRepo)
	linkedResourceSvc    := service.NewLinkedResourceService(linkedResourceRepo, boardRepo, activityLogger)
	flowSvc              := service.NewFlowService(flowRepo)
	flowVoteSvc          := service.NewFlowVoteService(flowVoteRepo, flowRepo)

	// Controllers
	healthCtrl       := controller.NewHealthController()
	authCtrl         := controller.NewAuthController(authSvc)
	userCtrl         := controller.NewUserController(userSvc)
	boardCtrl        := controller.NewBoardController(boardSvc)
	orgCtrl          := controller.NewOrgController(orgSvc)
	taskCtrl         := controller.NewTaskController(taskSvc)
	dashboardCtrl    := controller.NewDashboardController(dashboardSvc)
	activityCtrl     := controller.NewActivityController(activitySvc)
	analyticsCtrl    := controller.NewAnalyticsController(analyticsSvc)
	notificationCtrl    := controller.NewNotificationController(notificationSvc)
	linkedResourceCtrl  := controller.NewLinkedResourceController(linkedResourceSvc)
	flowCtrl            := controller.NewFlowController(flowSvc)
	flowVoteCtrl        := controller.NewFlowVoteController(flowVoteSvc)
	wsCtrl              := controller.NewWsController(hub, cfg.JWTSecret)

	// ── 4. Router ─────────────────────────────────────────────────────────────
	r := gin.Default()
	routes.SetupRoutes(r, cfg, healthCtrl, boardCtrl, authCtrl, userCtrl, orgCtrl, taskCtrl, dashboardCtrl, activityCtrl, analyticsCtrl, notificationCtrl, linkedResourceCtrl, flowCtrl, flowVoteCtrl, wsCtrl, store)

	// ── 5. Start server ───────────────────────────────────────────────────────
	addr := ":" + cfg.Port
	utils.Info(constants.LogTagMain, "starting server on "+addr)
	if err := r.Run(addr); err != nil {
		log.Fatalf("%s server failed: %v", constants.LogTagMain, err)
	}
}
