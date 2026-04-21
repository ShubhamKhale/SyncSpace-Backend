package service

import (
	"context"

	"syncspace-backend/model"
	"syncspace-backend/service/database"
)

// NotificationService handles notification retrieval and read-state management.
type NotificationService struct {
	repo *database.NotificationRepo
}

// NewNotificationService creates a NotificationService with the provided repository.
func NewNotificationService(repo *database.NotificationRepo) *NotificationService {
	return &NotificationService{repo: repo}
}

// GetNotifications returns a paginated notification list for the user.
// The response includes the total unread count across ALL pages (not just the
// current one), computed in a single database pass via window functions.
//
// limit is clamped to [1, 100]; offset must be ≥ 0.
func (s *NotificationService) GetNotifications(
	ctx context.Context,
	userID string,
	limit, offset int,
) (*model.NotificationPage, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}
	return s.repo.GetByUser(ctx, userID, limit, offset)
}

// MarkRead marks a single notification as read.
// Enforces ownership: only the notification's owner can mark it read.
func (s *NotificationService) MarkRead(ctx context.Context, notifID, userID string) error {
	return s.repo.MarkRead(ctx, notifID, userID)
}

// MarkAllRead marks every unread notification for the user as read.
// Returns the number of notifications that were updated.
func (s *NotificationService) MarkAllRead(ctx context.Context, userID string) (int64, error) {
	return s.repo.MarkAllRead(ctx, userID)
}
