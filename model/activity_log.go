package model

import "time"

// ActivityLog records an action taken on an entity (board, task, org, etc.).
// Metadata holds arbitrary key-value context (e.g. old/new field values).
type ActivityLog struct {
	ID         string         `json:"id"`
	EntityType string         `json:"entity_type"` // "board" | "task" | "org"
	EntityID   string         `json:"entity_id"`
	ActorID    string         `json:"actor_id"`
	Action     string         `json:"action"` // see ActivityLogger action constants
	Metadata   map[string]any `json:"metadata"`
	CreatedAt  time.Time      `json:"created_at"`
}

// ActivityLogDetail is ActivityLog enriched with the actor's display name,
// resolved via a JOIN at query time so callers never need a second lookup.
type ActivityLogDetail struct {
	ActivityLog
	ActorName string `json:"actor_name"`
}

// ActivityLogPage is the paginated envelope returned by the list endpoint.
// Total reflects all matching rows across all pages, computed via a window
// function in a single database pass.
type ActivityLogPage struct {
	Logs  []ActivityLogDetail `json:"logs"`
	Total int                 `json:"total"`
}
