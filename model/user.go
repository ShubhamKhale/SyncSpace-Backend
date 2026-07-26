package model

import "time"

// User represents an authenticated account in SyncSpace.
type User struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	Email        string    `json:"email"`
	AvatarURL    string    `json:"avatar_url"`
	PasswordHash string    `json:"-"` // never serialised in JSON responses
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// AuthUser is returned by auth endpoints and GET /api/user/me.
// Includes org membership fields the frontend uses to gate onboarding vs dashboard.
type AuthUser struct {
	ID        string  `json:"id"`
	Name      string  `json:"name"`
	Email     string  `json:"email"`
	AvatarURL *string `json:"avatar_url"`
	Role      *string `json:"role"`
	OrgID     *string `json:"orgId"`
	HasOrg    bool    `json:"hasOrg"`
}
