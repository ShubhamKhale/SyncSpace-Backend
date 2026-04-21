package model

import "time"

// LinkedResource is a URL bookmark attached to a board (design docs, Figma
// files, Notion pages, etc.).
type LinkedResource struct {
	ID        string    `json:"id"`
	BoardID   string    `json:"board_id"`
	Label     string    `json:"label"`     // human-readable name
	URL       string    `json:"url"`       // fully-qualified URL
	CreatedBy string    `json:"created_by"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
