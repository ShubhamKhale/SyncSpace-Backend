// Package model contains domain models — plain Go structs that represent
// the core entities of the SyncSpace application.
package model

import "time"

// Board represents a collaborative workspace in SyncSpace.
type Board struct {
	ID          string    `json:"id"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	OwnerID     string    `json:"owner_id"`
	OrgID       string    `json:"org_id"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// BoardHealth summarises a board's task risk profile, derived from its tasks.
type BoardHealth struct {
	Status       string `json:"status"` // healthy | warning | critical
	OverdueTasks int    `json:"overdueTasks"`
	Bottlenecks  int    `json:"bottlenecks"`
	AtRisk       int    `json:"atRisk"`
}

// BoardMember is one entry in the GET /boards/:id/members response.
// Role is mapped from org membership: owner | editor | viewer.
type BoardMember struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Email     string `json:"email"`
	Role      string `json:"role"`       // owner | editor | viewer
	AvatarURL string `json:"avatar_url"` // empty string if not set
}

// BoardActivityEntry is one row of board-scoped recent activity, with the
// action humanised and the affected entity's title resolved at query time.
type BoardActivityEntry struct {
	ID        string    `json:"id"`
	Actor     string    `json:"actor"`
	Action    string    `json:"action"`
	Target    string    `json:"target,omitempty"`
	CreatedAt time.Time `json:"createdAt"`
}
