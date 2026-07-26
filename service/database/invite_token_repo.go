package database

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"syncspace-backend/errs"
	"syncspace-backend/model"
)

// InviteTokenRepo handles database operations for invite tokens.
type InviteTokenRepo struct {
	db *pgxpool.Pool
}

// NewInviteTokenRepo creates an InviteTokenRepo with the provided connection pool.
func NewInviteTokenRepo(db *pgxpool.Pool) *InviteTokenRepo {
	return &InviteTokenRepo{db: db}
}

// InsertInviteToken persists a new invite token record.
func (r *InviteTokenRepo) InsertInviteToken(ctx context.Context, t *model.InviteToken) error {
	_, err := r.db.Exec(ctx,
		`INSERT INTO public.invite_tokens
		     (id, token, org_id, email, role, invited_by, expires_at, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		t.ID, t.Token, t.OrgID, t.Email, t.Role, t.InvitedBy, t.ExpiresAt, t.CreatedAt,
	)
	if err != nil {
		return errs.Internal("failed to insert invite token")
	}
	return nil
}

// GetValidInviteToken returns an invite token that has not been used and has not expired.
func (r *InviteTokenRepo) GetValidInviteToken(ctx context.Context, token string) (*model.InviteToken, error) {
	t := &model.InviteToken{}
	err := r.db.QueryRow(ctx,
		`SELECT id, token, org_id, email, role, invited_by, expires_at, used_at, created_at
		 FROM public.invite_tokens
		 WHERE token = $1 AND used_at IS NULL AND expires_at > NOW()`,
		token,
	).Scan(&t.ID, &t.Token, &t.OrgID, &t.Email, &t.Role, &t.InvitedBy, &t.ExpiresAt, &t.UsedAt, &t.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errs.BadRequest("Invalid or expired invite")
		}
		return nil, errs.Internal("failed to query invite token")
	}
	return t, nil
}

// GetInviteTokenWithOrg returns an invite token joined with its organization name.
// Used by the public verify endpoint.
func (r *InviteTokenRepo) GetInviteTokenWithOrg(ctx context.Context, token string) (*model.InviteToken, string, error) {
	t := &model.InviteToken{}
	var orgName string
	var usedAt *time.Time
	err := r.db.QueryRow(ctx,
		`SELECT it.id, it.token, it.org_id, it.email, it.role, it.invited_by,
		        it.expires_at, it.used_at, it.created_at, o.name
		 FROM   public.invite_tokens it
		 JOIN   public.organizations o ON o.id = it.org_id
		 WHERE  it.token = $1`,
		token,
	).Scan(
		&t.ID, &t.Token, &t.OrgID, &t.Email, &t.Role, &t.InvitedBy,
		&t.ExpiresAt, &usedAt, &t.CreatedAt, &orgName,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, "", nil // not_found — caller checks nil return
		}
		return nil, "", errs.Internal("failed to query invite token")
	}
	t.UsedAt = usedAt
	return t, orgName, nil
}

// MarkInviteUsed sets used_at = NOW() for the given token.
func (r *InviteTokenRepo) MarkInviteUsed(ctx context.Context, token string) error {
	_, err := r.db.Exec(ctx,
		`UPDATE public.invite_tokens SET used_at = NOW() WHERE token = $1`,
		token,
	)
	if err != nil {
		return errs.Internal("failed to mark invite as used")
	}
	return nil
}
