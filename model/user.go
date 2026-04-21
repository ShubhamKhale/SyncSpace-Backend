package model

import "time"

// User represents an authenticated account in SyncSpace.
type User struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	Email        string    `json:"email"`
	Bio          string    `json:"bio"`
	AvatarURL    string    `json:"avatar_url"`
	PasswordHash string    `json:"-"` // never serialised in JSON responses
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}
