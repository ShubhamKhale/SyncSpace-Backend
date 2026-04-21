package model

// DashboardStats is the aggregated overview returned by GET /api/dashboard/stats.
type DashboardStats struct {
	BoardCount     int `json:"board_count"`
	TaskTodo       int `json:"task_todo"`
	TaskInProgress int `json:"task_in_progress"`
	TaskDone       int `json:"task_done"`
	TaskDueSoon    int `json:"task_due_soon"` // due within 7 days and not done
}
