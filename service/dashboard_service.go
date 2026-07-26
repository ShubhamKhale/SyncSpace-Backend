package service

import (
	"context"

	"syncspace-backend/model"
	"syncspace-backend/service/database"
)

// DashboardService provides data for the dashboard overview endpoints.
type DashboardService struct {
	dashRepo  *database.DashboardRepo
	boardRepo *database.BoardRepo
	taskRepo  *database.TaskRepo
}

// NewDashboardService creates a DashboardService with the required repositories.
func NewDashboardService(
	dashRepo *database.DashboardRepo,
	boardRepo *database.BoardRepo,
	taskRepo *database.TaskRepo,
) *DashboardService {
	return &DashboardService{dashRepo: dashRepo, boardRepo: boardRepo, taskRepo: taskRepo}
}

// GetStats returns aggregated board and task counts for the org's dashboard.
func (s *DashboardService) GetStats(ctx context.Context, orgID string) (*model.DashboardStats, error) {
	return s.dashRepo.GetStats(ctx, orgID)
}

// GetRecentBoards returns the org's most recently updated boards.
func (s *DashboardService) GetRecentBoards(ctx context.Context, orgID string, limit int) ([]model.Board, error) {
	if limit <= 0 {
		limit = 5
	}
	if limit > 20 {
		limit = 20
	}
	return s.boardRepo.GetRecentBoardsByOrg(ctx, orgID, limit)
}

// GetUpcomingTasks returns non-done tasks assigned to or created by the user
// that are due within the next 7 days, ordered by due date ascending.
func (s *DashboardService) GetUpcomingTasks(ctx context.Context, userID string, limit int) ([]model.Task, error) {
	if limit <= 0 {
		limit = 10
	}
	if limit > 50 {
		limit = 50
	}
	return s.taskRepo.GetUpcomingTasksByUser(ctx, userID, limit)
}
