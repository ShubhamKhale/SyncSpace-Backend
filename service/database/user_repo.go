package database

import (
	"context"
	"errors"

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
		`INSERT INTO users (id, name, email, bio, avatar_url, password_hash, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		user.ID, user.Name, user.Email, user.Bio, user.AvatarURL,
		user.PasswordHash, user.CreatedAt, user.UpdatedAt,
	)
	if err != nil {
		var pgErr *pgconn.PgError
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
		`SELECT id, name, email, bio, avatar_url, password_hash, created_at, updated_at
		 FROM users WHERE id = $1`,
		id,
	).Scan(
		&user.ID, &user.Name, &user.Email, &user.Bio, &user.AvatarURL,
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
		`SELECT id, name, email, bio, avatar_url, password_hash, created_at, updated_at
		 FROM users WHERE email = $1`,
		email,
	).Scan(
		&user.ID, &user.Name, &user.Email, &user.Bio, &user.AvatarURL,
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

// UpdateUser persists updated profile fields (name, email, bio, avatar_url).
// Returns errs.Conflict if the new email is already taken.
func (r *UserRepo) UpdateUser(ctx context.Context, user *model.User) error {
	tag, err := r.db.Exec(ctx,
		`UPDATE users
		 SET name=$2, email=$3, bio=$4, avatar_url=$5, updated_at=$6
		 WHERE id=$1`,
		user.ID, user.Name, user.Email, user.Bio, user.AvatarURL, user.UpdatedAt,
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
