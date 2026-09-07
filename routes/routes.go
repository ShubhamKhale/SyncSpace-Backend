// Package routes wires HTTP routes to controller handlers.
package routes

import (
	"github.com/gin-gonic/gin"

	"syncspace-backend/config"
	"syncspace-backend/constants"
	"syncspace-backend/controller"
	"syncspace-backend/middleware"
	"syncspace-backend/service/database"
	"syncspace-backend/service/session"
)

// SetupRoutes registers all application routes on the provided Gin engine.
func SetupRoutes(
	r *gin.Engine,
	cfg *config.Config,
	orgRepo *database.OrgRepo,
	hc *controller.HealthController,
	bc *controller.BoardController,
	ac *controller.AuthController,
	uc *controller.UserController,
	oc *controller.OrgController,
	tc *controller.TaskController,
	dc *controller.DashboardController,
	alc *controller.ActivityController,
	anc *controller.AnalyticsController,
	nc *controller.NotificationController,
	lrc *controller.LinkedResourceController,
	fc *controller.FlowController,
	fvc *controller.FlowVoteController,
	bfc  *controller.BoardFlowController,
	bfvc *controller.BoardFlowVoteController,
	prc  *controller.PresenceController,
	wsc *controller.WsController,
	aic *controller.AIController,
	dgc *controller.DiagramController,
	store *session.Store,
) {
	r.GET(constants.URIHealth, hc.GetHealth)
	r.GET(constants.URIWebSocket, wsc.ServeWs)

	api := r.Group("/api")
	{
		// ── Public auth routes ────────────────────────────────────────────────
		auth := api.Group("/auth")
		{
			auth.POST(constants.URIRegister, ac.Register)
			auth.POST(constants.URISignIn, ac.SignIn)
			auth.POST(constants.URILogout, middleware.JWTAuth(cfg.JWTSecret), ac.Logout)
		}

		// ── Public org routes (no JWT) ────────────────────────────────────────
		api.GET(constants.URIOrgInviteVerify, oc.VerifyInvite)

		// ── JWT-only routes ───────────────────────────────────────────────────
		jwtOnly := api.Group("/")
		jwtOnly.Use(middleware.JWTAuth(cfg.JWTSecret))
		jwtOnly.Use(middleware.OrgContext(orgRepo))
		{
			// User profile
			jwtOnly.GET(constants.URIUserMe, uc.GetMe)
			jwtOnly.PATCH(constants.URIUserProfile, uc.UpdateProfile)
			jwtOnly.POST(constants.URIUserAvatar, uc.UploadAvatar)
			jwtOnly.PATCH(constants.URIUserAvatar, uc.UpdateAvatar)

			// User settings
			jwtOnly.GET(constants.URISettingsNotifications, uc.GetNotificationPrefs)
			jwtOnly.PATCH(constants.URISettingsNotifications, uc.UpdateNotificationPrefs)

			// Organization — core
			// NOTE: static invite routes must be registered before param routes
			jwtOnly.POST(constants.URIOrganization, oc.CreateOrganization)
			jwtOnly.GET(constants.URIOrganization, oc.GetOrganization)
			jwtOnly.PATCH(constants.URIOrganizationID, oc.UpdateOrganization)

			// Organization — invite token flow
			jwtOnly.POST(constants.URIOrgInvite, oc.SendInvite)

			// Organization — members
			jwtOnly.GET(constants.URIOrgMembers, oc.GetMembers)
			jwtOnly.POST(constants.URIOrgMembersInvite, oc.InviteMember)
			jwtOnly.PATCH(constants.URIOrgMemberRole, oc.UpdateMemberRole)
			jwtOnly.DELETE(constants.URIOrgMemberID, oc.RemoveMember)

			// Board CRUD (static /boards/recent registered above; :id routes registered after)
			jwtOnly.GET(constants.URIBoardByID, bc.GetBoardByID)
			jwtOnly.PATCH(constants.URIBoardByID, bc.UpdateBoard)

			// Board sub-resources
			jwtOnly.GET(constants.URIBoardLinkedResources, lrc.GetLinkedResources)
			jwtOnly.PUT(constants.URIBoardLinkedResources, lrc.ReplaceLinkedResources)
			jwtOnly.GET(constants.URIBoardHealth, bc.GetBoardHealth)
			jwtOnly.GET(constants.URIBoardActivity, bc.GetBoardActivity)
			jwtOnly.GET(constants.URIBoardMembers, bc.GetBoardMembers)

			// Tasks
			jwtOnly.GET(constants.URIBoardTasks, tc.GetTasks)
			jwtOnly.POST(constants.URIBoardTasks, tc.CreateTask)
			jwtOnly.GET(constants.URITasksUpcoming, dc.GetUpcomingTasks)
			jwtOnly.PATCH(constants.URITaskStage, tc.MoveTask)
			jwtOnly.PATCH(constants.URITaskByID, tc.UpdateTask)

			// Dashboard
			jwtOnly.GET(constants.URIDashboardStats, dc.GetStats)
			jwtOnly.GET(constants.URIBoardsRecent, dc.GetRecentBoards)

			// Activity logs
			jwtOnly.GET(constants.URIActivityLogs, alc.GetActivityLogs)

			// Notifications
			jwtOnly.GET(constants.URINotifications, nc.GetNotifications)
			jwtOnly.PATCH(constants.URINotificationsMarkAll, nc.MarkAllRead)
			jwtOnly.PATCH(constants.URINotificationRead, nc.MarkRead)

			// Analytics
			jwtOnly.GET(constants.URIAnalyticsTaskCompletion, anc.GetTaskCompletionTrend)
			jwtOnly.GET(constants.URIAnalyticsTaskCompletionTrend, anc.GetTaskCompletionTrendMonthly)
			jwtOnly.GET(constants.URIAnalyticsTaskDistribution, anc.GetTaskDistribution)
			jwtOnly.GET(constants.URIAnalyticsBoardActivity, anc.GetBoardActivity)
			jwtOnly.GET(constants.URIAnalyticsTeamContribution, anc.GetTeamContribution)
			jwtOnly.GET(constants.URIAnalyticsDashboard, anc.GetDashboardAnalytics)

			// Flow diagrams (legacy single-flow per board)
			jwtOnly.GET(constants.URIFlowVotes, fvc.GetVotes)
			jwtOnly.POST(constants.URIFlowVotes, fvc.CastVote)
			jwtOnly.GET(constants.URIFlowByID, fc.GetFlow)
			jwtOnly.PUT(constants.URIFlowByID, fc.UpdateFlow)

			// Flow presence (REST — static join/leave before param routes)
			jwtOnly.POST(constants.URIFlowParticipantsJoin, prc.Join)
			jwtOnly.GET(constants.URIFlowParticipants, prc.List)
			jwtOnly.DELETE(constants.URIFlowParticipantsLeave, prc.Leave)

			// Board flow diagrams (multi-flow CRUD + duplicate + diagram data)
			jwtOnly.GET(constants.URIBoardFlows, bfc.ListFlows)
			jwtOnly.POST(constants.URIBoardFlows, bfc.CreateFlow)
			jwtOnly.GET(constants.URIBoardFlowByID, bfc.GetFlow)
			jwtOnly.PUT(constants.URIBoardFlowByID, bfc.RenameFlow)
			jwtOnly.DELETE(constants.URIBoardFlowByID, bfc.DeleteFlow)
			jwtOnly.POST(constants.URIBoardFlowDuplicate, bfc.DuplicateFlow)
			jwtOnly.GET(constants.URIBoardFlowDiagram, bfc.GetDiagram)
			jwtOnly.PUT(constants.URIBoardFlowDiagram, bfc.SaveDiagram)

			// Board flow votes
			jwtOnly.GET(constants.URIBoardFlowVotes, bfvc.GetVotes)
			jwtOnly.POST(constants.URIBoardFlowVotes, bfvc.CastVote)
			jwtOnly.DELETE(constants.URIBoardFlowVotes, bfvc.RemoveVote)

			// AI — local model chat + summarization
			jwtOnly.POST(constants.URIBoardAIChat, aic.ChatWithBoard)
			jwtOnly.POST(constants.URIAISummarize, aic.Summarize)

			// AI — diagram generation (Groq compound model only, no Ollama fallback)
			jwtOnly.POST(constants.URIAIGenerateDiagram, dgc.GenerateDiagram)
		}

		// ── Protected routes: JWT + OrgContext + AES encryption ──────────────
		protected := api.Group("/")
		protected.Use(middleware.JWTAuth(cfg.JWTSecret))
		protected.Use(middleware.OrgContext(orgRepo))
		protected.Use(middleware.Encryption(store))
		{
			protected.POST(constants.URIBoards, bc.CreateBoard)
			protected.GET(constants.URIBoards, bc.GetBoards)
		}
	}
}
