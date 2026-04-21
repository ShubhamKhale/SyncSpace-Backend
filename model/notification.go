package model

import "time"

// Notification is a message delivered to a specific user when something
// noteworthy happens (e.g. a task is assigned, a member is invited).
type Notification struct {
	ID         string    `json:"id"`
	UserID     string    `json:"user_id"`
	Title      string    `json:"title"`
	Body       string    `json:"body"`
	IsRead     bool      `json:"is_read"`
	EntityType string    `json:"entity_type,omitempty"` // "task" | "board" | "org" …
	EntityID   string    `json:"entity_id,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
}

// NotificationPage is the envelope returned by the list endpoint.
// UnreadCount reflects ALL unread notifications for the user (not just this page),
// computed in a single query pass via a PostgreSQL window function.
type NotificationPage struct {
	Notifications []Notification `json:"notifications"`
	UnreadCount   int            `json:"unread_count"`
	Total         int            `json:"total"`
}
