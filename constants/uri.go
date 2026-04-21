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

	// Organization routes
	URIOrganization          = "/organization"
	URIOrganizationID        = "/organization/:id"
	URIOrgMembers            = "/organization/members"
	URIOrgMembersInvite      = "/organization/members/invite"
	URIOrgMemberRole         = "/organization/members/:id/role"
	URIOrgMemberID           = "/organization/members/:id"

	// Board sub-resource routes (share :id param with board tasks)
	URIBoardLinkedResources = "/boards/:id/linked-resources"

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
	URIAnalyticsTaskCompletion   = "/analytics/task-completion"
	URIAnalyticsTaskDistribution = "/analytics/task-distribution"
	URIAnalyticsBoardActivity    = "/analytics/board-activity"
	URIAnalyticsTeamContribution = "/analytics/team-contribution"

	// Flow diagram routes
	URIFlowByID  = "/flows/:id"
	URIFlowVotes = "/flows/:id/votes"

	// WebSocket
	URIWebSocket = "/ws"

	// Protected API routes (JWT + AES encryption)
	URIBoards = "/boards"
)
