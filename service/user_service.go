package service

import (
	"context"
	"time"

	"syncspace-backend/errs"
	"syncspace-backend/model"
	"syncspace-backend/service/database"
)

// UserService handles user profile business logic.
type UserService struct {
	repo *database.UserRepo
}

// NewUserService creates a UserService backed by the given UserRepo.
func NewUserService(repo *database.UserRepo) *UserService {
	return &UserService{repo: repo}
}

// GetProfile returns the full profile for the authenticated user.
func (s *UserService) GetProfile(ctx context.Context, userID string) (*model.User, error) {
	return s.repo.GetUserByID(ctx, userID)
}

// UpdateProfile applies partial updates to name, email, and bio.
// Only non-nil pointer fields are applied; others keep their current value.
func (s *UserService) UpdateProfile(ctx context.Context, userID string, name, email, bio *string) (*model.User, error) {
	user, err := s.repo.GetUserByID(ctx, userID)
	if err != nil {
		return nil, err
	}

	if name != nil {
		if *name == "" {
			return nil, errs.BadRequest("name cannot be empty")
		}
		user.Name = *name
	}
	if email != nil {
		if *email == "" {
			return nil, errs.BadRequest("email cannot be empty")
		}
		user.Email = *email
	}
	if bio != nil {
		user.Bio = *bio
	}

	user.UpdatedAt = time.Now()

	if err := s.repo.UpdateUser(ctx, user); err != nil {
		return nil, err
	}
	return user, nil
}

// UpdateAvatar stores a new avatar URL for the user.
// The URL is accepted as-is (mock / pre-signed URL from the client).
func (s *UserService) UpdateAvatar(ctx context.Context, userID, avatarURL string) (*model.User, error) {
	if avatarURL == "" {
		return nil, errs.BadRequest("avatar_url cannot be empty")
	}

	user, err := s.repo.GetUserByID(ctx, userID)
	if err != nil {
		return nil, err
	}

	user.AvatarURL = avatarURL
	user.UpdatedAt = time.Now()

	if err := s.repo.UpdateUser(ctx, user); err != nil {
		return nil, err
	}
	return user, nil
}
