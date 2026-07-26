package model

import "time"

// Subtask is a checklist item nested inside a Task.
type Subtask struct {
	ID        string `json:"id"`
	Text      string `json:"text"`
	Completed bool   `json:"completed"`
}

// Attachment holds metadata for a file attached to a Task.
// Binary is stored externally (S3/GCS); only the URL reference is persisted here.
type Attachment struct {
	ID         string    `json:"id"`
	Name       string    `json:"name"`
	URL        string    `json:"url"`
	Type       string    `json:"type,omitempty"`
	Size       int64     `json:"size,omitempty"`
	UploadedAt time.Time `json:"uploaded_at"`
}

// Task is a unit of work that belongs to a board.
//
// Valid stages:     Planning | Design | Development | QA | Deployment
// Valid priorities: low | medium | high
type Task struct {
	ID              string       `json:"id"`
	BoardID         string       `json:"board_id"`
	Title           string       `json:"title"`
	Description     string       `json:"description"`
	Stage           string       `json:"stage"`
	Priority        string       `json:"priority"`
	AssigneeID      string       `json:"assignee_id,omitempty"`
	CreatedBy       string       `json:"created_by"`
	Position        int          `json:"position"`
	DueDate         *time.Time   `json:"due_date,omitempty"`
	StartDate       *time.Time   `json:"start_date,omitempty"`
	Tags            []string     `json:"tags"`
	TimeEstimate    string       `json:"time_estimate,omitempty"`
	ReferenceLink   string       `json:"reference_link,omitempty"`
	FlowDiagramLink string       `json:"flow_diagram_link,omitempty"`
	Subtasks        []Subtask    `json:"subtasks"`
	Attachments     []Attachment `json:"attachments"`
	CreatedAt       time.Time    `json:"created_at"`
	UpdatedAt       time.Time    `json:"updated_at"`
}
