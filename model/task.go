package model

import "time"

// Task is a unit of work that belongs to a board.
//
// Valid stages:    "todo" | "in_progress" | "done"
// Valid priorities: "low" | "medium" | "high"
type Task struct {
	ID          string     `json:"id"`
	BoardID     string     `json:"board_id"`
	Title       string     `json:"title"`
	Description string     `json:"description"`
	Stage       string     `json:"stage"`               // todo | in_progress | done
	Priority    string     `json:"priority"`             // low | medium | high
	AssigneeID  string     `json:"assignee_id,omitempty"`
	CreatedBy   string     `json:"created_by"`
	Position    int        `json:"position"`             // 0-based order within a stage column
	DueDate     *time.Time `json:"due_date,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}
