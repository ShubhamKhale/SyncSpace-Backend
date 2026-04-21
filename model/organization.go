package model

import "time"

// Organization is a top-level workspace that groups boards and members.
type Organization struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	OwnerID     string    `json:"owner_id"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// OrgMember represents a membership record inside an organization.
//
// Valid roles:   "admin" | "member" | "viewer"   (owner is tracked via organizations.owner_id)
// Valid statuses: "active" | "invited"
type OrgMember struct {
	OrgID     string    `json:"org_id"`
	UserID    string    `json:"user_id"`
	Role      string    `json:"role"`
	Status    string    `json:"status"`              // active | invited
	InvitedBy string    `json:"invited_by,omitempty"` // user_id of inviter
	JoinedAt  time.Time `json:"joined_at"`
}

// OrgMemberDetail is returned in list / invite responses — it merges the
// membership record with the user's public profile fields.
type OrgMemberDetail struct {
	UserID    string    `json:"user_id"`
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	AvatarURL string    `json:"avatar_url"`
	Role      string    `json:"role"`
	Status    string    `json:"status"`
	InvitedBy string    `json:"invited_by,omitempty"`
	JoinedAt  time.Time `json:"joined_at"`
}

// OrgWithRole is returned in list responses so the caller can see their own
// role alongside the organization details.
type OrgWithRole struct {
	Organization
	Role string `json:"role"` // caller's role in this org
}
