// main is the application entry point.
package main

import (
	"context"
	"log"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"

	"syncspace-backend/config"
	"syncspace-backend/constants"
	"syncspace-backend/controller"
	"syncspace-backend/pkg/aiclient"
	"syncspace-backend/pkg/db"
	pkgcloudinary "syncspace-backend/pkg/cloudinary"
	pkgemail "syncspace-backend/pkg/email"
	"syncspace-backend/pkg/groq"
	"syncspace-backend/pkg/ollama"
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

	// ── 2b. Redis (optional) ──────────────────────────────────────────────────
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
	store := session.NewStore()

	// Repos
	userRepo            := database.NewUserRepo(db.Pool)
	userPrefsRepo       := database.NewUserPrefsRepo(db.Pool)
	boardRepo           := database.NewBoardRepo(db.Pool)
	orgRepo             := database.NewOrgRepo(db.Pool)
	inviteTokenRepo     := database.NewInviteTokenRepo(db.Pool)
	taskRepo            := database.NewTaskRepo(db.Pool)
	dashboardRepo       := database.NewDashboardRepo(db.Pool)
	activityRepo        := database.NewActivityRepo(db.Pool)
	analyticsRepo       := database.NewAnalyticsRepo(db.Pool)
	notificationRepo    := database.NewNotificationRepo(db.Pool)
	linkedResourceRepo  := database.NewLinkedResourceRepo(db.Pool)
	flowRepo            := database.NewFlowRepo(db.Pool)
	flowVoteRepo        := database.NewFlowVoteRepo(db.Pool)
	boardFlowVoteRepo   := database.NewBoardFlowVoteRepo(db.Pool)

	activityLogger := service.NewActivityLogger(activityRepo)

	// Email
	emailTemplateRepo := database.NewEmailTemplateRepo(db.Pool)
	var mailer *pkgemail.Mailer
	if cfg.BrevoAPIKey != "" {
		mailer = pkgemail.NewMailer(cfg.BrevoAPIKey, cfg.BrevoFromEmail, cfg.BrevoFromName)
		utils.Info("[EMAIL]", "Brevo mailer initialised from="+cfg.BrevoFromEmail)
	} else {
		utils.Info("[EMAIL]", "BREVO_API_KEY not set — invite emails disabled")
	}

	// Cloudinary
	var cldUploader *pkgcloudinary.Uploader
	if cfg.CloudinaryCloudName != "" {
		var err error
		cldUploader, err = pkgcloudinary.NewUploader(cfg.CloudinaryCloudName, cfg.CloudinaryAPIKey, cfg.CloudinaryAPISecret)
		if err != nil {
			log.Fatalf("[CLOUDINARY] failed to init: %v", err)
		}
		utils.Info("[CLOUDINARY]", "uploader initialised cloud="+cfg.CloudinaryCloudName)
	} else {
		utils.Info("[CLOUDINARY]", "CLOUDINARY_CLOUD_NAME not set — avatar upload disabled")
	}

	// Services
	authSvc          := service.NewAuthService(userRepo, orgRepo, inviteTokenRepo, store, cfg.JWTSecret)
	userSvc          := service.NewUserService(userRepo, userPrefsRepo)
	boardSvc         := service.NewBoardService(boardRepo, activityLogger)
	orgSvc           := service.NewOrgService(orgRepo, userRepo, inviteTokenRepo, emailTemplateRepo, mailer, cfg.FrontendURL)
	taskSvc          := service.NewTaskService(taskRepo, activityLogger)
	dashboardSvc     := service.NewDashboardService(dashboardRepo, boardRepo, taskRepo)
	activitySvc      := service.NewActivityService(activityRepo)
	analyticsSvc     := service.NewAnalyticsService(analyticsRepo)
	notificationSvc  := service.NewNotificationService(notificationRepo)
	linkedResourceSvc := service.NewLinkedResourceService(linkedResourceRepo, boardRepo, activityLogger)
	flowSvc          := service.NewFlowService(flowRepo)
	flowVoteSvc      := service.NewFlowVoteService(flowVoteRepo, flowRepo)
	boardFlowSvc     := service.NewBoardFlowService(database.NewBoardFlowRepo(db.Pool), boardRepo)
	boardFlowVoteSvc := service.NewBoardFlowVoteService(boardFlowVoteRepo, boardRepo)
	var presenceSvc  *service.PresenceService
	if pkgredis.Client != nil {
		presenceSvc = service.NewPresenceService(pkgredis.Client, userRepo)
	}

	// AI — Groq in prod (GROQ_API_KEY set), local Ollama in dev otherwise.
	var llmClient aiclient.ChatClient
	var groqClient *groq.Client // kept for diagram generation, which is Groq-only (needs groq/compound's web search)
	if cfg.GroqAPIKey != "" {
		groqClient = groq.NewClient(cfg.GroqAPIKey, cfg.GroqModel)
		llmClient = groqClient
		utils.Info("[AI]", "using Groq model="+cfg.GroqModel)
	} else {
		llmClient = ollama.NewClient(cfg.OllamaURL, cfg.OllamaModel)
		utils.Info("[AI]", "GROQ_API_KEY not set — using local Ollama model="+cfg.OllamaModel)
	}
	aiSvc := service.NewAIService(llmClient, boardRepo, taskRepo, linkedResourceRepo)

	// Diagram generation — always Groq's compound model regardless of the
	// chat/summarize provider above; no offline fallback (needs web search).
	if groqClient == nil {
		utils.Info("[AI]", "GROQ_API_KEY not set — diagram generation disabled")
	}
	diagramSvc := service.NewDiagramService(groqClient)

	// Controllers
	healthCtrl          := controller.NewHealthController()
	authCtrl            := controller.NewAuthController(authSvc)
	userCtrl            := controller.NewUserController(userSvc, cldUploader)
	boardCtrl           := controller.NewBoardController(boardSvc)
	orgCtrl             := controller.NewOrgController(orgSvc)
	taskCtrl            := controller.NewTaskController(taskSvc)
	dashboardCtrl       := controller.NewDashboardController(dashboardSvc)
	activityCtrl        := controller.NewActivityController(activitySvc)
	analyticsCtrl       := controller.NewAnalyticsController(analyticsSvc)
	notificationCtrl    := controller.NewNotificationController(notificationSvc)
	linkedResourceCtrl  := controller.NewLinkedResourceController(linkedResourceSvc)
	flowCtrl            := controller.NewFlowController(flowSvc)
	flowVoteCtrl        := controller.NewFlowVoteController(flowVoteSvc)
	boardFlowCtrl       := controller.NewBoardFlowController(boardFlowSvc, hub)
	boardFlowVoteCtrl   := controller.NewBoardFlowVoteController(boardFlowVoteSvc)
	presenceCtrl        := controller.NewPresenceController(presenceSvc)
	wsCtrl              := controller.NewWsController(hub, cfg.JWTSecret)
	aiCtrl              := controller.NewAIController(aiSvc)
	diagramCtrl         := controller.NewDiagramController(diagramSvc)

	// ── 4. Router ─────────────────────────────────────────────────────────────
	gin.SetMode(cfg.GinMode)
	r := gin.Default()
	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{cfg.FrontendURL},
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization", "Accept", "Cache-Control", "X-Requested-With", "X-User-ID"},
		ExposeHeaders:    []string{"Content-Length", "Content-Type"},
		AllowCredentials: true,
		MaxAge:           12 * 60 * 60, // 12h in seconds
	}))
	routes.SetupRoutes(
		r, cfg, orgRepo,
		healthCtrl, boardCtrl, authCtrl, userCtrl, orgCtrl,
		taskCtrl, dashboardCtrl, activityCtrl, analyticsCtrl,
		notificationCtrl, linkedResourceCtrl, flowCtrl, flowVoteCtrl,
		boardFlowCtrl, boardFlowVoteCtrl, presenceCtrl, wsCtrl, aiCtrl, diagramCtrl, store,
	)

	// ── 5. Start server ───────────────────────────────────────────────────────
	addr := ":" + cfg.Port
	utils.Info(constants.LogTagMain, "starting server on "+addr)
	if err := r.Run(addr); err != nil {
		log.Fatalf("%s server failed: %v", constants.LogTagMain, err)
	}
}
