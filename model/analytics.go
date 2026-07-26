package model

// ── Task completion trend ─────────────────────────────────────────────────────

// TaskTrendPoint is one day's task creation and completion count, used to
// render a line/bar chart over a chosen time window (week / month / quarter).
type TaskTrendPoint struct {
	Date      string `json:"date"`      // YYYY-MM-DD
	Created   int    `json:"created"`   // tasks created on this day
	Completed int    `json:"completed"` // tasks moved to 'done' on this day
}

// ── Task distribution ─────────────────────────────────────────────────────────

// StageCount holds the number of tasks currently in a given stage.
type StageCount struct {
	Stage string `json:"stage"`
	Count int    `json:"count"`
}

// PriorityCount holds the number of tasks at a given priority level.
type PriorityCount struct {
	Priority string `json:"priority"`
	Count    int    `json:"count"`
}

// TaskDistribution groups current task counts by stage and by priority.
// Both slices are always returned; empty stages/priorities are omitted.
type TaskDistribution struct {
	ByStage    []StageCount    `json:"by_stage"`
	ByPriority []PriorityCount `json:"by_priority"`
}

// ── Board activity ────────────────────────────────────────────────────────────

// BoardActivityPoint is one day's task-level activity within a board (or
// across all boards owned by the user).
type BoardActivityPoint struct {
	Date         string `json:"date"`          // YYYY-MM-DD
	TasksCreated int    `json:"tasks_created"` // new tasks opened this day
	TasksUpdated int    `json:"tasks_updated"` // existing tasks touched this day
}

// ── Team contribution ─────────────────────────────────────────────────────────

// TeamMemberContribution summarises one user's task activity within the
// scoped board(s): how many they created, how many are done, how many are
// overdue (past due_date and not done).
type TeamMemberContribution struct {
	UserID    string `json:"user_id"`
	UserName  string `json:"user_name"`
	Created   int    `json:"tasks_created"`
	Completed int    `json:"tasks_completed"`
	Overdue   int    `json:"tasks_overdue"`
}

// ── Dashboard analytics (new shapes) ─────────────────────────────────────────

// MonthlyTrendPoint is one month's task creation and completion count.
// Used by GET /api/analytics/task-completion-trend.
type MonthlyTrendPoint struct {
	Month     string `json:"month"`     // "Jan", "Feb", etc.
	Created   int    `json:"created"`
	Completed int    `json:"completed"`
}

// DistributionItem is a name+value pair used by the task-distribution pie chart.
type DistributionItem struct {
	Name  string `json:"name"`
	Value int    `json:"value"`
}

// BoardActivityItem holds per-board activity counts for the bar chart.
type BoardActivityItem struct {
	Name     string `json:"name"`
	Edits    int    `json:"edits"`
	Comments int    `json:"comments"`
	Shares   int    `json:"shares"`
}

// TeamContributionResult is the radar-chart response for GET /api/analytics/team-contribution.
// Data rows have dynamic member-name keys, so map[string]any is used.
type TeamContributionResult struct {
	Data    []map[string]any `json:"data"`
	Members []string         `json:"members"`
}

// DashboardAnalytics bundles all four datasets for GET /api/analytics/dashboard.
type DashboardAnalytics struct {
	TaskCompletionTrend []MonthlyTrendPoint    `json:"taskCompletionTrend"`
	TaskDistribution    []DistributionItem     `json:"taskDistribution"`
	BoardActivity       []BoardActivityItem    `json:"boardActivity"`
	TeamContribution    TeamContributionResult `json:"teamContribution"`
}
