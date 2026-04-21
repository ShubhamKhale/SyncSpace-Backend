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
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}
