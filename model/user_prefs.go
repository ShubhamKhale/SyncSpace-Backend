package model

import "time"

// NotificationPrefs holds per-user notification settings.
// Absent DB row → all fields use their defaults (comments/invites true, product_updates false).
type NotificationPrefs struct {
	Comments       bool      `json:"comments"`
	Invites        bool      `json:"invites"`
	ProductUpdates bool      `json:"product_updates"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// DefaultNotificationPrefs returns the canonical defaults used when no row exists.
func DefaultNotificationPrefs() *NotificationPrefs {
	return &NotificationPrefs{
		Comments:       true,
		Invites:        true,
		ProductUpdates: false,
	}
}
