package service

import (
	"context"
	"time"

	"syncspace-backend/errs"
	"syncspace-backend/model"
	"syncspace-backend/service/database"
)

// ── Permission maps ───────────────────────────────────────────────────────────

// orgEditRoles may update the organization record itself.
var orgEditRoles = map[string]bool{"owner": true, "admin": true}

// memberManageRoles may invite, role-change, and remove other members.
var memberManageRoles = map[string]bool{"owner": true, "admin": true}

// validAssignableRoles are the roles that can be assigned via the API.
// "owner" is intentionally excluded — ownership transfer is a separate operation.
var validAssignableRoles = map[string]bool{
	"admin":  true,
	"member": true,
	"viewer": true,
}

// OrgService handles organization and membership business logic.
type OrgService struct {
	repo     *database.OrgRepo
	userRepo *database.UserRepo // needed for invite email lookup
}

// NewOrgService creates an OrgService.
func NewOrgService(repo *database.OrgRepo, userRepo *database.UserRepo) *OrgService {
	return &OrgService{repo: repo, userRepo: userRepo}
}

// ── Organization operations ───────────────────────────────────────────────────

// GetMyOrgs returns all organizations the caller belongs to with their role.
func (s *OrgService) GetMyOrgs(ctx context.Context, callerID string) ([]model.OrgWithRole, error) {
	orgs, err := s.repo.GetOrgsByUserID(ctx, callerID)
	if err != nil {
		return nil, err
	}
	if orgs == nil {
		orgs = []model.OrgWithRole{}
	}
	return orgs, nil
}

// UpdateOrg updates the name and/or description of an organization.
// Only owner / admin may call this.
func (s *OrgService) UpdateOrg(ctx context.Context, callerID, orgID string, name, description *string) (*model.Organization, error) {
	if err := s.requireRole(ctx, orgID, callerID, orgEditRoles, "only owners and admins can update the organization"); err != nil {
		return nil, err
	}

	org, err := s.repo.GetOrgByID(ctx, orgID)
	if err != nil {
		return nil, err
	}

	if name != nil {
		if *name == "" {
			return nil, errs.BadRequest("organization name cannot be empty")
		}
		org.Name = *name
	}
	if description != nil {
		org.Description = *description
	}
	org.UpdatedAt = time.Now()

	if err := s.repo.UpdateOrg(ctx, org); err != nil {
		return nil, err
	}
	return org, nil
}

// ── Member operations ─────────────────────────────────────────────────────────

// GetMembers returns all members of an org.
// Any active member of the org may list members.
func (s *OrgService) GetMembers(ctx context.Context, callerID, orgID string) ([]model.OrgMemberDetail, error) {
	if _, err := s.repo.GetMemberRole(ctx, orgID, callerID); err != nil {
		return nil, err // 404 if caller is not a member
	}

	members, err := s.repo.GetOrgMembers(ctx, orgID)
	if err != nil {
		return nil, err
	}
	if members == nil {
		members = []model.OrgMemberDetail{}
	}
	return members, nil
}

// InviteMember looks up the target user by email, validates the role, and
// creates a membership record with status "invited".
// Only owner / admin may invite.
func (s *OrgService) InviteMember(ctx context.Context, callerID, orgID, email, role string) (*model.OrgMemberDetail, error) {
	// ── Permission ────────────────────────────────────────────────────────────
	if err := s.requireRole(ctx, orgID, callerID, memberManageRoles, "only owners and admins can invite members"); err != nil {
		return nil, err
	}

	// ── Role validation ───────────────────────────────────────────────────────
	if !validAssignableRoles[role] {
		return nil, errs.BadRequest("role must be one of: admin, member, viewer")
	}

	// ── Look up target user ───────────────────────────────────────────────────
	user, err := s.userRepo.GetUserByEmail(ctx, email)
	if err != nil {
		return nil, errs.NotFound("no user found with that email address")
	}

	// ── Insert membership ─────────────────────────────────────────────────────
	member := &model.OrgMember{
		OrgID:     orgID,
		UserID:    user.ID,
		Role:      role,
		Status:    "invited",
		InvitedBy: callerID,
		JoinedAt:  time.Now(),
	}
	if err := s.repo.InsertOrgMember(ctx, member); err != nil {
		return nil, err // Conflict if already a member
	}

	detail := &model.OrgMemberDetail{
		UserID:    user.ID,
		Name:      user.Name,
		Email:     user.Email,
		AvatarURL: user.AvatarURL,
		Role:      member.Role,
		Status:    member.Status,
		InvitedBy: member.InvitedBy,
		JoinedAt:  member.JoinedAt,
	}
	return detail, nil
}

// UpdateMemberRole changes the role of targetUserID inside orgID.
//
// Rules enforced:
//   - Caller must be owner or admin.
//   - New role must be a valid assignable role (admin | member | viewer).
//   - The org owner's role cannot be changed via this endpoint.
//   - An admin cannot promote another member to admin (only owner can).
func (s *OrgService) UpdateMemberRole(ctx context.Context, callerID, orgID, targetUserID, newRole string) error {
	// ── Caller permission ─────────────────────────────────────────────────────
	callerRole, err := s.repo.GetMemberRole(ctx, orgID, callerID)
	if err != nil {
		return err
	}
	if !memberManageRoles[callerRole] {
		return errs.Unauthorized("only owners and admins can change member roles")
	}

	// ── Role validation ───────────────────────────────────────────────────────
	if !validAssignableRoles[newRole] {
		return errs.BadRequest("role must be one of: admin, member, viewer")
	}

	// ── Guard: cannot change the owner ───────────────────────────────────────
	targetRole, err := s.repo.GetMemberRole(ctx, orgID, targetUserID)
	if err != nil {
		return err
	}
	if targetRole == "owner" {
		return errs.BadRequest("the organization owner's role cannot be changed")
	}

	// ── Guard: admin cannot promote to admin (only owner can) ────────────────
	if callerRole == "admin" && newRole == "admin" {
		return errs.Unauthorized("only the owner can grant admin role")
	}

	return s.repo.UpdateOrgMemberRole(ctx, orgID, targetUserID, newRole)
}

// RemoveMember removes targetUserID from the organization.
//
// Rules enforced:
//   - Caller is owner/admin, OR caller is removing themselves (self-leave).
//   - The org owner cannot be removed.
func (s *OrgService) RemoveMember(ctx context.Context, callerID, orgID, targetUserID string) error {
	// ── Guard: cannot remove the owner ────────────────────────────────────────
	targetRole, err := s.repo.GetMemberRole(ctx, orgID, targetUserID)
	if err != nil {
		return err
	}
	if targetRole == "owner" {
		return errs.BadRequest("the organization owner cannot be removed")
	}

	// ── Permission: self-leave is always allowed; otherwise must be owner/admin ─
	if callerID != targetUserID {
		callerRole, err := s.repo.GetMemberRole(ctx, orgID, callerID)
		if err != nil {
			return err
		}
		if !memberManageRoles[callerRole] {
			return errs.Unauthorized("only owners and admins can remove members")
		}
	}

	return s.repo.DeleteOrgMember(ctx, orgID, targetUserID)
}

// ── Private helpers ───────────────────────────────────────────────────────────

// requireRole fetches the caller's role and returns an Unauthorized error if it
// is not in the allowed set.
func (s *OrgService) requireRole(ctx context.Context, orgID, callerID string, allowed map[string]bool, msg string) error {
	role, err := s.repo.GetMemberRole(ctx, orgID, callerID)
	if err != nil {
		return err
	}
	if !allowed[role] {
		return errs.Unauthorized(msg)
	}
	return nil
}
