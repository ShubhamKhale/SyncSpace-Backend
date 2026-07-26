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
	repo      *database.UserRepo
	prefsRepo *database.UserPrefsRepo
}

// NewUserService creates a UserService backed by the given repos.
func NewUserService(repo *database.UserRepo, prefsRepo *database.UserPrefsRepo) *UserService {
	return &UserService{repo: repo, prefsRepo: prefsRepo}
}

// GetProfile returns the full profile for the authenticated user, including org membership.
func (s *UserService) GetProfile(ctx context.Context, userID string) (*model.AuthUser, error) {
	return s.repo.GetUserWithOrg(ctx, userID)
}

// UpdateProfile applies partial updates to name, email.
func (s *UserService) UpdateProfile(ctx context.Context, userID string, name, email *string) (*model.User, error) {
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

	user.UpdatedAt = time.Now()

	if err := s.repo.UpdateUser(ctx, user); err != nil {
		return nil, err
	}
	return user, nil
}

// UpdateAvatar stores a new avatar URL for the user.
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

// GetNotificationPrefs returns the current user's notification preferences.
// Returns defaults if no preferences have been saved yet.
func (s *UserService) GetNotificationPrefs(ctx context.Context, userID string) (*model.NotificationPrefs, error) {
	prefs, err := s.prefsRepo.GetByUserID(ctx, userID)
	if err != nil {
		return nil, errs.Internal("failed to fetch notification preferences")
	}
	return prefs, nil
}

// UpdateNotificationPrefs upserts notification preferences for the user.
func (s *UserService) UpdateNotificationPrefs(ctx context.Context, userID string, prefs *model.NotificationPrefs) (*model.NotificationPrefs, error) {
	result, err := s.prefsRepo.Upsert(ctx, userID, prefs)
	if err != nil {
		return nil, errs.Internal("failed to save notification preferences")
	}
	return result, nil
}
