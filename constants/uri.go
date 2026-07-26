package constants

// URI path constants used in both the router and controller layers to avoid
// hard-coding strings in multiple places.
const (
	URIHealth = "/health"

	// Public auth routes (no middleware)
	URIRegister = "/register"
	URISignIn   = "/signin"
	URILogin    = "/login"  // legacy
	URILogout   = "/logout"

	// User profile routes
	URIUserMe      = "/user/me"
	URIUserProfile = "/user/profile"
	URIUserAvatar  = "/user/avatar"

	// Settings routes
	URISettingsNotifications = "/settings/notifications"

	// Organization routes
	URIOrganization          = "/organization"
	URIOrganizationID        = "/organization/:id"
	URIOrgMembers            = "/organization/members"
	URIOrgMembersInvite      = "/organization/members/invite"
	URIOrgMemberRole         = "/organization/members/:id/role"
	URIOrgMemberID           = "/organization/members/:id"

	// Invite token routes
	URIOrgInvite       = "/organization/invite"         // POST — create invite token
	URIOrgInviteVerify = "/organization/invite/verify"  // GET  — public verify

	// Board CRUD + sub-resource routes (:id param; register static /boards/recent first)
	URIBoardByID            = "/boards/:id"
	URIBoardLinkedResources = "/boards/:id/linked-resources"
	URIBoardHealth          = "/boards/:id/health"
	URIBoardActivity        = "/boards/:id/activity"
	URIBoardMembers          = "/boards/:id/members"
	URIBoardFlows          = "/boards/:id/flows"
	URIBoardFlowByID       = "/boards/:id/flows/:flowId"
	URIBoardFlowDuplicate  = "/boards/:id/flows/:flowId/duplicate"
	URIBoardFlowDiagram    = "/boards/:id/flows/:flowId/diagram"
	URIBoardFlowVotes      = "/boards/:id/flows/:flowId/votes"

	// Task routes
	URIBoardTasks   = "/boards/:id/tasks" // nested: list + create
	URITaskByID     = "/tasks/:id"        // update fields
	URITaskStage    = "/tasks/:id/stage"  // move (drag-drop)
	URITasksUpcoming = "/tasks/upcoming"  // due within 7 days (register before :id)

	// Dashboard routes
	URIDashboardStats = "/dashboard/stats"
	URIBoardsRecent   = "/boards/recent"   // recent boards (register before :id)
	URIActivityLogs   = "/activity-logs"

	// Notification routes
	// NOTE: URINotificationsMarkAll (static) must be registered before URINotificationRead (param).
	URINotifications        = "/notifications"
	URINotificationsMarkAll = "/notifications/mark-all-read" // static — register first
	URINotificationRead     = "/notifications/:id/read"

	// Analytics routes
	URIAnalyticsTaskCompletion        = "/analytics/task-completion"
	URIAnalyticsTaskCompletionTrend   = "/analytics/task-completion-trend"
	URIAnalyticsTaskDistribution      = "/analytics/task-distribution"
	URIAnalyticsBoardActivity         = "/analytics/board-activity"
	URIAnalyticsTeamContribution      = "/analytics/team-contribution"
	URIAnalyticsDashboard             = "/analytics/dashboard"

	// Flow diagram routes
	URIFlowByID  = "/flows/:id"
	URIFlowVotes = "/flows/:id/votes"

	// Flow presence (REST — complement to WebSocket flow.join/leave events)
	// Static /join and /leave registered before any param routes.
	URIFlowParticipantsJoin  = "/boards/:id/flows/:flowId/participants/join"
	URIFlowParticipants      = "/boards/:id/flows/:flowId/participants"
	URIFlowParticipantsLeave = "/boards/:id/flows/:flowId/participants/leave"

	// WebSocket
	URIWebSocket = "/ws"

	// Protected API routes (JWT + AES encryption)
	URIBoards = "/boards"
)
