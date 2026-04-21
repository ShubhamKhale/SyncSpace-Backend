// Package routes wires HTTP routes to controller handlers.
// All path strings are sourced from the constants package to avoid duplication.
package routes

import (
	"github.com/gin-gonic/gin"

	"syncspace-backend/config"
	"syncspace-backend/constants"
	"syncspace-backend/controller"
	"syncspace-backend/middleware"
	"syncspace-backend/service/session"
)

// SetupRoutes registers all application routes on the provided Gin engine.
//
// Route groups:
//   - GET    /health                                   — liveness probe, no auth
//   - POST   /api/auth/register                       — create account; returns JWT + session key
//   - POST   /api/auth/signin                         — validate credentials; returns JWT + session key
//   - POST   /api/auth/logout                         — revoke session key; requires JWT
//   - GET    /api/user/me                             — current user profile; requires JWT
//   - PATCH  /api/user/profile                        — update name/email/bio; requires JWT
//   - PATCH  /api/user/avatar                         — update avatar URL; requires JWT
//   - GET    /api/organization                        — list caller's organizations; requires JWT
//   - PATCH  /api/organization/:id                    — update org (owner/admin); requires JWT
//   - GET    /api/organization/members                — list members; requires JWT
//   - POST   /api/organization/members/invite         — invite member; requires JWT
//   - PATCH  /api/organization/members/:id/role       — update member role; requires JWT
//   - DELETE /api/organization/members/:id            — remove member; requires JWT
//   - GET    /api/boards/:id/tasks                    — list tasks for a board; requires JWT
//   - POST   /api/boards/:id/tasks                    — create task; requires JWT
//   - GET    /api/tasks/upcoming                      — tasks due within 7 days; requires JWT
//   - PATCH  /api/tasks/:id/stage                     — move task (drag-drop); requires JWT
//   - PATCH  /api/tasks/:id                           — update task fields; requires JWT
//   - GET    /api/dashboard/stats                     — aggregated overview; requires JWT
//   - GET    /api/boards/recent                       — recently updated boards; requires JWT
//   - GET    /api/activity-logs                       — recent activity; requires JWT
//   - GET    /api/notifications                       — list with unread count; requires JWT
//   - PATCH  /api/notifications/mark-all-read         — mark all read; requires JWT
//   - PATCH  /api/notifications/:id/read              — mark one read; requires JWT
//   - GET    /api/analytics/task-completion           — daily trend; requires JWT
//   - GET    /api/analytics/task-distribution         — by stage & priority; requires JWT
//   - GET    /api/analytics/board-activity            — daily activity; requires JWT
//   - GET    /api/analytics/team-contribution         — per-member stats; requires JWT
//   - GET    /api/flows/:id                           — get flow diagram; requires JWT
//   - PUT    /api/flows/:id                           — update flow diagram (optimistic lock); requires JWT
//   - POST   /api/flows/:id/votes                     — cast or update a vote; requires JWT
//   - GET    /api/flows/:id/votes                     — vote summary + caller's vote; requires JWT
//   - GET    /ws                                      — WebSocket; ?token=<JWT> required
//   - POST   /api/boards                              — create board; JWT + AES encryption
//   - GET    /api/boards                              — list boards; JWT + AES encryption
func SetupRoutes(
	r *gin.Engine,
	cfg *config.Config,
	hc  *controller.HealthController,
	bc  *controller.BoardController,
	ac  *controller.AuthController,
	uc  *controller.UserController,
	oc  *controller.OrgController,
	tc  *controller.TaskController,
	dc  *controller.DashboardController,
	alc *controller.ActivityController,
	anc *controller.AnalyticsController,
	nc  *controller.NotificationController,
	lrc  *controller.LinkedResourceController,
	fc   *controller.FlowController,
	fvc  *controller.FlowVoteController,
	wsc  *controller.WsController,
	store *session.Store,
) {
	// Liveness probe — no auth required
	r.GET(constants.URIHealth, hc.GetHealth)

	// WebSocket — auth via ?token= query param (JWT); no middleware needed
	r.GET(constants.URIWebSocket, wsc.ServeWs)

	api := r.Group("/api")
	{
		// ── Public auth routes (no JWT, no encryption) ────────────────────────
		auth := api.Group("/auth")
		{
			auth.POST(constants.URIRegister, ac.Register)
			auth.POST(constants.URISignIn, ac.SignIn)
			auth.POST(constants.URILogout, middleware.JWTAuth(cfg.JWTSecret), ac.Logout)
		}

		// ── JWT-only routes (plaintext JSON) ──────────────────────────────────
		jwtOnly := api.Group("/")
		jwtOnly.Use(middleware.JWTAuth(cfg.JWTSecret))
		{
			// User profile
			jwtOnly.GET(constants.URIUserMe, uc.GetMe)
			jwtOnly.PATCH(constants.URIUserProfile, uc.UpdateProfile)
			jwtOnly.PATCH(constants.URIUserAvatar, uc.UpdateAvatar)

			// Organization — core
			jwtOnly.GET(constants.URIOrganization, oc.GetOrganization)
			jwtOnly.PATCH(constants.URIOrganizationID, oc.UpdateOrganization)

			// Organization — members
			// NOTE: static routes (invite) must be registered before param routes (:id)
			jwtOnly.GET(constants.URIOrgMembers, oc.GetMembers)
			jwtOnly.POST(constants.URIOrgMembersInvite, oc.InviteMember)
			jwtOnly.PATCH(constants.URIOrgMemberRole, oc.UpdateMemberRole)
			jwtOnly.DELETE(constants.URIOrgMemberID, oc.RemoveMember)

			// Board sub-resources (linked resources)
			jwtOnly.GET(constants.URIBoardLinkedResources, lrc.GetLinkedResources)
			jwtOnly.PUT(constants.URIBoardLinkedResources, lrc.ReplaceLinkedResources)

			// Tasks — board-scoped CRUD
			// NOTE: static suffixes registered before param routes to prevent shadowing
			jwtOnly.GET(constants.URIBoardTasks, tc.GetTasks)
			jwtOnly.POST(constants.URIBoardTasks, tc.CreateTask)
			jwtOnly.GET(constants.URITasksUpcoming, dc.GetUpcomingTasks) // static before /tasks/:id
			jwtOnly.PATCH(constants.URITaskStage, tc.MoveTask)           // static "stage" before /tasks/:id
			jwtOnly.PATCH(constants.URITaskByID, tc.UpdateTask)

			// Dashboard
			jwtOnly.GET(constants.URIDashboardStats, dc.GetStats)
			jwtOnly.GET(constants.URIBoardsRecent, dc.GetRecentBoards)

			// Activity logs
			jwtOnly.GET(constants.URIActivityLogs, alc.GetActivityLogs)

			// Notifications
			// NOTE: URINotificationsMarkAll (static) registered before URINotificationRead (param)
			jwtOnly.GET(constants.URINotifications, nc.GetNotifications)
			jwtOnly.PATCH(constants.URINotificationsMarkAll, nc.MarkAllRead) // static first
			jwtOnly.PATCH(constants.URINotificationRead, nc.MarkRead)

			// Analytics — read-only aggregation
			jwtOnly.GET(constants.URIAnalyticsTaskCompletion,   anc.GetTaskCompletionTrend)
			jwtOnly.GET(constants.URIAnalyticsTaskDistribution, anc.GetTaskDistribution)
			jwtOnly.GET(constants.URIAnalyticsBoardActivity,    anc.GetBoardActivity)
			jwtOnly.GET(constants.URIAnalyticsTeamContribution, anc.GetTeamContribution)

			// Flow diagrams
			// NOTE: URIFlowVotes (static suffix "votes") registered before URIFlowByID (:id)
			jwtOnly.GET(constants.URIFlowVotes, fvc.GetVotes)
			jwtOnly.POST(constants.URIFlowVotes, fvc.CastVote)
			jwtOnly.GET(constants.URIFlowByID, fc.GetFlow)
			jwtOnly.PUT(constants.URIFlowByID, fc.UpdateFlow)
		}

		// ── Protected routes: JWT → AES encryption ────────────────────────────
		protected := api.Group("/")
		protected.Use(middleware.JWTAuth(cfg.JWTSecret))
		protected.Use(middleware.Encryption(store))
		{
			protected.POST(constants.URIBoards, bc.CreateBoard)
			protected.GET(constants.URIBoards, bc.GetBoards)
		}
	}
}
