package model

import "time"

// InviteToken represents a one-time email invite link for joining an organization.
type InviteToken struct {
	ID        string     `json:"id"`
	Token     string     `json:"token"`
	OrgID     string     `json:"org_id"`
	Email     string     `json:"email"`
	Role      string     `json:"role"`
	InvitedBy string     `json:"invited_by"`
	ExpiresAt time.Time  `json:"expires_at"`
	UsedAt    *time.Time `json:"used_at,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
	OrgName   string     `json:"org_name,omitempty"` // populated by join query for verify endpoint
}
