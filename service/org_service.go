package service

import (
	"bytes"
	"context"
	"html/template"
	"time"

	"syncspace-backend/errs"
	"syncspace-backend/model"
	"syncspace-backend/pkg/email"
	"syncspace-backend/pkg/utils"
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

// InviteVerifyResult is returned by VerifyInvite.
type InviteVerifyResult struct {
	Valid   bool   `json:"valid"`
	OrgName string `json:"orgName,omitempty"`
	Role    string `json:"role,omitempty"`
	Email   string `json:"email,omitempty"`
	Reason  string `json:"reason,omitempty"` // "expired" | "used" | "not_found"
}

// OrgService handles organization and membership business logic.
type OrgService struct {
	repo         *database.OrgRepo
	userRepo     *database.UserRepo
	inviteRepo   *database.InviteTokenRepo
	templateRepo *database.EmailTemplateRepo
	mailer       *email.Mailer // nil when BREVO_API_KEY is not set
	frontendURL  string
}

// NewOrgService creates an OrgService.
func NewOrgService(
	repo *database.OrgRepo,
	userRepo *database.UserRepo,
	inviteRepo *database.InviteTokenRepo,
	templateRepo *database.EmailTemplateRepo,
	mailer *email.Mailer,
	frontendURL string,
) *OrgService {
	return &OrgService{
		repo:         repo,
		userRepo:     userRepo,
		inviteRepo:   inviteRepo,
		templateRepo: templateRepo,
		mailer:       mailer,
		frontendURL:  frontendURL,
	}
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

// ── New org operations ────────────────────────────────────────────────────────

// CreateOrg creates a new organization owned by callerID.
// Returns errs.Conflict if the caller already belongs to an org.
func (s *OrgService) CreateOrg(ctx context.Context, callerID, name string) (*model.Organization, error) {
	if name == "" {
		return nil, errs.BadRequest("organization name cannot be empty")
	}

	_, _, found, err := s.repo.GetUserPrimaryOrgID(ctx, callerID)
	if err != nil {
		return nil, err
	}
	if found {
		return nil, errs.Conflict("User already belongs to an organization")
	}

	now := time.Now()
	org := &model.Organization{
		ID:        utils.NewUUID(),
		Name:      name,
		OwnerID:   callerID,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := s.repo.InsertOrganization(ctx, org); err != nil {
		return nil, err
	}

	// Insert owner into org_members so the LEFT JOIN in GetUserWithOrg works.
	member := &model.OrgMember{
		OrgID:    org.ID,
		UserID:   callerID,
		Role:     "owner",
		Status:   "active",
		JoinedAt: now,
	}
	if err := s.repo.InsertOrgMember(ctx, member); err != nil {
		return nil, err
	}

	return org, nil
}

// SendInvite creates an invite token for the given email to join the caller's org.
func (s *OrgService) SendInvite(ctx context.Context, callerID, email, role string) (*model.InviteToken, error) {
	if role == "owner" {
		return nil, errs.BadRequest("Cannot invite as owner")
	}
	if !validAssignableRoles[role] {
		return nil, errs.BadRequest("role must be one of: admin, member, viewer")
	}

	orgID, callerRole, found, err := s.repo.GetUserPrimaryOrgID(ctx, callerID)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, errs.BadRequest("you must belong to an organization to send invites")
	}
	if !memberManageRoles[callerRole] {
		return nil, errs.Forbidden("Insufficient permissions")
	}

	now := time.Now()
	invite := &model.InviteToken{
		ID:        utils.NewUUID(),
		Token:     utils.NewUUID(),
		OrgID:     orgID,
		Email:     email,
		Role:      role,
		InvitedBy: callerID,
		ExpiresAt: now.Add(7 * 24 * time.Hour),
		CreatedAt: now,
	}
	if err := s.inviteRepo.InsertInviteToken(ctx, invite); err != nil {
		return nil, err
	}

	if s.mailer != nil {
		go s.sendInviteEmail(invite, callerID)
	}
	return invite, nil
}

// sendInviteEmail fires a rendered invite email in a goroutine.
// All errors are logged and swallowed — never propagated to the caller.
func (s *OrgService) sendInviteEmail(invite *model.InviteToken, callerID string) {
	ctx := context.Background()

	caller, err := s.userRepo.GetUserByID(ctx, callerID)
	if err != nil {
		utils.Error("[EMAIL]", "sendInviteEmail: resolve caller", err)
		return
	}

	org, err := s.repo.GetOrgByID(ctx, invite.OrgID)
	if err != nil {
		utils.Error("[EMAIL]", "sendInviteEmail: resolve org", err)
		return
	}

	tpl, err := s.templateRepo.Get(ctx, "org_invite")
	if err != nil {
		utils.Error("[EMAIL]", "sendInviteEmail: load template", err)
		return
	}

	data := struct {
		OrgName        string
		InviterName    string
		Role           string
		InviteLink     string
		RecipientEmail string
	}{
		OrgName:        org.Name,
		InviterName:    caller.Name,
		Role:           invite.Role,
		InviteLink:     s.frontendURL + "/invite?token=" + invite.Token,
		RecipientEmail: invite.Email,
	}

	renderedSubject, err := renderTemplate("subject", tpl.Subject, data)
	if err != nil {
		utils.Error("[EMAIL]", "sendInviteEmail: render subject", err)
		return
	}

	renderedHTML, err := renderTemplate("html_body", tpl.HTMLBody, data)
	if err != nil {
		utils.Error("[EMAIL]", "sendInviteEmail: render html", err)
		return
	}

	if err := s.mailer.Send(ctx, invite.Email, renderedSubject, renderedHTML); err != nil {
		utils.Error("[EMAIL]", "sendInviteEmail: send failed to "+invite.Email, err)
		return
	}

	utils.Info("[EMAIL]", "invite email sent to "+invite.Email)
}

// renderTemplate parses and executes a Go html/template string against data.
func renderTemplate(name, src string, data any) (string, error) {
	t, err := template.New(name).Parse(src)
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// VerifyInvite checks the validity of an invite token without consuming it.
func (s *OrgService) VerifyInvite(ctx context.Context, token string) (*InviteVerifyResult, error) {
	invite, orgName, err := s.inviteRepo.GetInviteTokenWithOrg(ctx, token)
	if err != nil {
		return nil, err
	}
	if invite == nil {
		return &InviteVerifyResult{Valid: false, Reason: "not_found"}, nil
	}
	if invite.UsedAt != nil {
		return &InviteVerifyResult{Valid: false, Reason: "used"}, nil
	}
	if invite.ExpiresAt.Before(time.Now()) {
		return &InviteVerifyResult{Valid: false, Reason: "expired"}, nil
	}
	return &InviteVerifyResult{
		Valid:   true,
		OrgName: orgName,
		Role:    invite.Role,
		Email:   invite.Email,
	}, nil
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
