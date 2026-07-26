package database

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"syncspace-backend/errs"
	"syncspace-backend/model"
)

// OrgRepo handles all database operations for organizations and their members.
type OrgRepo struct {
	db *pgxpool.Pool
}

// NewOrgRepo creates an OrgRepo with the provided connection pool.
func NewOrgRepo(db *pgxpool.Pool) *OrgRepo {
	return &OrgRepo{db: db}
}

// ── Organization queries ──────────────────────────────────────────────────────

// GetOrgsByUserID returns every organization the user belongs to, together
// with their effective role (owner | admin | member | viewer).
func (r *OrgRepo) GetOrgsByUserID(ctx context.Context, userID string) ([]model.OrgWithRole, error) {
	rows, err := r.db.Query(ctx,
		`SELECT o.id, o.name, o.description, o.owner_id, o.created_at, o.updated_at,
		        CASE WHEN o.owner_id = $1 THEN 'owner' ELSE om.role END AS role
		 FROM   public.organizations o
		 JOIN   public.organization_members om ON om.organization_id = o.id AND om.user_id = $1
		 ORDER  BY o.created_at ASC`,
		userID,
	)
	if err != nil {
		fmt.Println("GetOrgsByUserID query error for user " + userID + ": " + err.Error())
		return nil, errs.Internal("failed to query organizations")
	}
	defer rows.Close()

	var orgs []model.OrgWithRole
	for rows.Next() {
		var o model.OrgWithRole
		if err := rows.Scan(
			&o.ID, &o.Name, &o.Description, &o.OwnerID,
			&o.CreatedAt, &o.UpdatedAt, &o.Role,
		); err != nil {
			return nil, errs.Internal("failed to scan organization row")
		}
		orgs = append(orgs, o)
	}
	if rows.Err() != nil {
		return nil, errs.Internal("organization query error")
	}
	return orgs, nil
}

// GetOrgByID returns a single organization by its ID, or errs.NotFound.
func (r *OrgRepo) GetOrgByID(ctx context.Context, id string) (*model.Organization, error) {
	org := &model.Organization{}
	err := r.db.QueryRow(ctx,
		`SELECT id, name, description, owner_id, created_at, updated_at
		 FROM   public.organizations WHERE id = $1`,
		id,
	).Scan(&org.ID, &org.Name, &org.Description, &org.OwnerID, &org.CreatedAt, &org.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errs.NotFound("organization not found")
		}
		return nil, errs.Internal("failed to query organization")
	}
	return org, nil
}

// UpdateOrg persists the name and description of an existing organization.
func (r *OrgRepo) UpdateOrg(ctx context.Context, org *model.Organization) error {
	tag, err := r.db.Exec(ctx,
		`UPDATE public.organizations
		 SET    name=$2, description=$3, updated_at=$4
		 WHERE  id=$1`,
		org.ID, org.Name, org.Description, org.UpdatedAt,
	)
	if err != nil {
		return errs.Internal("failed to update organization")
	}
	if tag.RowsAffected() == 0 {
		return errs.NotFound("organization not found")
	}
	return nil
}

// ── Membership queries ────────────────────────────────────────────────────────

// GetMemberRole returns the effective role of userID in orgID.
// The owner always returns "owner". Returns errs.NotFound if not a member.
func (r *OrgRepo) GetMemberRole(ctx context.Context, orgID, userID string) (string, error) {
	var ownerID string
	err := r.db.QueryRow(ctx,
		`SELECT owner_id FROM public.organizations WHERE id = $1`, orgID,
	).Scan(&ownerID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", errs.NotFound("organization not found")
		}
		return "", errs.Internal("failed to query organization")
	}
	if ownerID == userID {
		return "owner", nil
	}

	var role string
	err = r.db.QueryRow(ctx,
		`SELECT role FROM public.organization_members
		 WHERE  organization_id = $1 AND user_id = $2`,
		orgID, userID,
	).Scan(&role)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", errs.NotFound("user is not a member of this organization")
		}
		return "", errs.Internal("failed to query membership")
	}
	return role, nil
}

// GetOrgMembers returns all members of an organization joined with their
// public profile fields (name, email, avatar_url).
func (r *OrgRepo) GetOrgMembers(ctx context.Context, orgID string) ([]model.OrgMemberDetail, error) {
	rows, err := r.db.Query(ctx,
		`SELECT om.user_id, u.name, u.email, u.avatar_url,
		        om.role, om.status, COALESCE(om.invited_by,''), om.joined_at
		 FROM   public.organization_members om
		 JOIN   public.users u ON u.id = om.user_id
		 WHERE  om.organization_id = $1
		 ORDER  BY om.joined_at ASC`,
		orgID,
	)
	if err != nil {
		return nil, errs.Internal("failed to query members")
	}
	defer rows.Close()

	var members []model.OrgMemberDetail
	for rows.Next() {
		var m model.OrgMemberDetail
		if err := rows.Scan(
			&m.UserID, &m.Name, &m.Email, &m.AvatarURL,
			&m.Role, &m.Status, &m.InvitedBy, &m.JoinedAt,
		); err != nil {
			return nil, errs.Internal("failed to scan member row")
		}
		members = append(members, m)
	}
	if rows.Err() != nil {
		return nil, errs.Internal("member query error")
	}
	return members, nil
}

// GetOrgMember returns a single membership record, or errs.NotFound.
func (r *OrgRepo) GetOrgMember(ctx context.Context, orgID, userID string) (*model.OrgMember, error) {
	m := &model.OrgMember{OrgID: orgID, UserID: userID}
	err := r.db.QueryRow(ctx,
		`SELECT role, status, COALESCE(invited_by,''), joined_at
		 FROM   public.organization_members
		 WHERE  organization_id = $1 AND user_id = $2`,
		orgID, userID,
	).Scan(&m.Role, &m.Status, &m.InvitedBy, &m.JoinedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errs.NotFound("member not found")
		}
		return nil, errs.Internal("failed to query member")
	}
	return m, nil
}

// InsertOrgMember adds a new membership row. Returns errs.Conflict if the
// user is already a member of this organization.
func (r *OrgRepo) InsertOrgMember(ctx context.Context, m *model.OrgMember) error {
	_, err := r.db.Exec(ctx,
		`INSERT INTO public.organization_members
		     (organization_id, user_id, role, status, invited_by, joined_at)
		 VALUES ($1, $2, $3, $4, NULLIF($5,''), $6)`,
		m.OrgID, m.UserID, m.Role, m.Status, m.InvitedBy, m.JoinedAt,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return errs.Conflict("user is already a member of this organization")
		}
		return errs.Internal("failed to insert member")
	}
	return nil
}

// UpdateOrgMemberRole changes the role of an existing member.
func (r *OrgRepo) UpdateOrgMemberRole(ctx context.Context, orgID, userID, role string) error {
	tag, err := r.db.Exec(ctx,
		`UPDATE public.organization_members
		 SET    role = $3
		 WHERE  organization_id = $1 AND user_id = $2`,
		orgID, userID, role,
	)
	if err != nil {
		return errs.Internal("failed to update member role")
	}
	if tag.RowsAffected() == 0 {
		return errs.NotFound("member not found")
	}
	return nil
}

// GetUserPrimaryOrgID returns the org ID and role for the user's active membership.
// Returns found=false (not an error) when the user has no active org membership.
func (r *OrgRepo) GetUserPrimaryOrgID(ctx context.Context, userID string) (orgID, role string, found bool, err error) {
	err = r.db.QueryRow(ctx,
		`SELECT om.organization_id,
		        CASE WHEN o.owner_id = $1 THEN 'owner' ELSE om.role END AS role
		 FROM   public.organization_members om
		 JOIN   public.organizations o ON o.id = om.organization_id
		 WHERE  om.user_id = $1 AND om.status = 'active'
		 LIMIT  1`,
		userID,
	).Scan(&orgID, &role)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", "", false, nil
		}
		return "", "", false, errs.Internal("failed to query org membership")
	}
	return orgID, role, true, nil
}

// InsertOrganization persists a new organization record.
func (r *OrgRepo) InsertOrganization(ctx context.Context, org *model.Organization) error {
	_, err := r.db.Exec(ctx,
		`INSERT INTO public.organizations (id, name, description, owner_id, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6)`,
		org.ID, org.Name, org.Description, org.OwnerID, org.CreatedAt, org.UpdatedAt,
	)
	if err != nil {
		return errs.Internal("failed to create organization")
	}
	return nil
}

// DeleteOrgMember removes a membership record from the organization.
func (r *OrgRepo) DeleteOrgMember(ctx context.Context, orgID, userID string) error {
	tag, err := r.db.Exec(ctx,
		`DELETE FROM public.organization_members
		 WHERE  organization_id = $1 AND user_id = $2`,
		orgID, userID,
	)
	if err != nil {
		return errs.Internal("failed to remove member")
	}
	if tag.RowsAffected() == 0 {
		return errs.NotFound("member not found")
	}
	return nil
}
