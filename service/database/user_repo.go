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

// UserRepo handles all database operations for the User entity.
type UserRepo struct {
	db *pgxpool.Pool
}

// NewUserRepo creates a new UserRepo with the provided connection pool.
func NewUserRepo(db *pgxpool.Pool) *UserRepo {
	return &UserRepo{db: db}
}

// InsertUser persists a new user. Returns errs.Conflict on duplicate email.
func (r *UserRepo) InsertUser(ctx context.Context, user *model.User) error {

	_, err := r.db.Exec(ctx,
		`INSERT INTO public.users (id, name, email, avatar_url, password_hash, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		user.ID, user.Name, user.Email, user.AvatarURL,
		user.PasswordHash, user.CreatedAt, user.UpdatedAt,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		fmt.Println("error:", err.Error())
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return errs.Conflict("email is already registered")
		}
		return errs.Internal("failed to create user")
	}
	return nil
}

// GetUserByID returns the user with the given ID, or errs.NotFound.
func (r *UserRepo) GetUserByID(ctx context.Context, id string) (*model.User, error) {
	user := &model.User{}
	err := r.db.QueryRow(ctx,
		`SELECT id, name, email, avatar_url, password_hash, created_at, updated_at
		 FROM public.users WHERE id = $1`,
		id,
	).Scan(
		&user.ID, &user.Name, &user.Email, &user.AvatarURL,
		&user.PasswordHash, &user.CreatedAt, &user.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errs.NotFound("user not found")
		}
		return nil, errs.Internal("failed to query user")
	}
	return user, nil
}

// GetUserByEmail returns the user with the given email, or errs.NotFound.
func (r *UserRepo) GetUserByEmail(ctx context.Context, email string) (*model.User, error) {
	user := &model.User{}
	err := r.db.QueryRow(ctx,
		`SELECT id, name, email, avatar_url, password_hash, created_at, updated_at
		 FROM public.users WHERE email = $1`,
		email,
	).Scan(
		&user.ID, &user.Name, &user.Email, &user.AvatarURL,
		&user.PasswordHash, &user.CreatedAt, &user.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errs.NotFound("user not found")
		}
		return nil, errs.Internal("failed to query user")
	}
	return user, nil
}

// GetUserWithOrg returns the authenticated user's profile plus their active org membership.
// Used by auth and profile endpoints to build the AuthUser response shape.
func (r *UserRepo) GetUserWithOrg(ctx context.Context, userID string) (*model.AuthUser, error) {
	u := &model.AuthUser{}
	var avatarURL, role, orgID *string
	var hasOrg bool
	err := r.db.QueryRow(ctx,
		`SELECT u.id, u.name, u.email, u.avatar_url,
		        CASE WHEN o.owner_id = u.id THEN 'owner' ELSE om.role END AS role,
		        om.organization_id,
		        (om.organization_id IS NOT NULL) AS has_org
		 FROM   public.users u
		 LEFT JOIN public.organization_members om
		        ON om.user_id = u.id AND om.status = 'active'
		 LEFT JOIN public.organizations o
		        ON o.id = om.organization_id
		 WHERE  u.id = $1
		 LIMIT  1`,
		userID,
	).Scan(&u.ID, &u.Name, &u.Email, &avatarURL, &role, &orgID, &hasOrg)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errs.NotFound("user not found")
		}
		return nil, errs.Internal("failed to query user")
	}
	u.AvatarURL = avatarURL
	u.Role = role
	u.OrgID = orgID
	u.HasOrg = hasOrg
	return u, nil
}

// UpdateUser persists updated profile fields (name, email, avatar_url).
// Returns errs.Conflict if the new email is already taken.
func (r *UserRepo) UpdateUser(ctx context.Context, user *model.User) error {
	tag, err := r.db.Exec(ctx,
		`UPDATE public.users
		 SET name=$2, email=$3, avatar_url=$4, updated_at=$5
		 WHERE id=$1`,
		user.ID, user.Name, user.Email, user.AvatarURL, user.UpdatedAt,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return errs.Conflict("email is already in use")
		}
		return errs.Internal("failed to update user")
	}
	if tag.RowsAffected() == 0 {
		return errs.NotFound("user not found")
	}
	return nil
}
