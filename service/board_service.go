// Package service contains the application's business-logic layer.
package service

import (
	"context"
	"time"

	"syncspace-backend/errs"
	"syncspace-backend/model"
	"syncspace-backend/pkg/utils"
	"syncspace-backend/service/database"
	"syncspace-backend/service/operation"
)

// BoardService orchestrates board-related use cases.
type BoardService struct {
	repo   *database.BoardRepo
	logger *ActivityLogger
}

// NewBoardService creates a BoardService wired to the provided repository.
func NewBoardService(repo *database.BoardRepo, logger *ActivityLogger) *BoardService {
	return &BoardService{repo: repo, logger: logger}
}

// CreateBoard validates the input, builds a Board model, and persists it.
func (s *BoardService) CreateBoard(ctx context.Context, ownerID, orgID, title, description string) (*model.Board, error) {
	if err := operation.ValidateCreateBoard(title, description); err != nil {
		return nil, err
	}

	now := time.Now()
	board := &model.Board{
		ID:          utils.NewUUID(),
		Title:       title,
		Description: description,
		OwnerID:     ownerID,
		OrgID:       orgID,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	if err := s.repo.InsertBoard(ctx, board); err != nil {
		return nil, err
	}

	s.logger.LogBoard(ctx, ownerID, board.ID, ActionCreated, map[string]any{
		"title": title,
	})
	return board, nil
}

// GetBoardsByOrg retrieves all boards for the given org.
func (s *BoardService) GetBoardsByOrg(ctx context.Context, orgID string) ([]model.Board, error) {
	if orgID == "" {
		return nil, errs.BadRequest("org ID is required")
	}
	return s.repo.GetBoardsByOrg(ctx, orgID)
}

// GetRecentBoards returns up to limit boards for the org, sorted by last update.
func (s *BoardService) GetRecentBoards(ctx context.Context, orgID string, limit int) ([]model.Board, error) {
	if limit <= 0 {
		limit = 5
	}
	if limit > 20 {
		limit = 20
	}
	return s.repo.GetRecentBoardsByOrg(ctx, orgID, limit)
}

// GetBoard returns a single board by ID, scoped to the caller's org.
func (s *BoardService) GetBoard(ctx context.Context, boardID, orgID string) (*model.Board, error) {
	return s.repo.GetBoardByIDAndOrg(ctx, boardID, orgID)
}

// UpdateBoard applies a partial update (title and/or description) to a board.
// The board must belong to the given org. At least one field must be non-nil.
func (s *BoardService) UpdateBoard(ctx context.Context, boardID, orgID string, title, description *string) (*model.Board, error) {
	if title != nil && *title == "" {
		return nil, errs.BadRequest("title cannot be empty")
	}
	return s.repo.UpdateBoard(ctx, boardID, orgID, title, description)
}

// GetBoardHealth verifies the board belongs to the caller's org and computes a
// risk-based health summary from its tasks.
func (s *BoardService) GetBoardHealth(ctx context.Context, boardID, orgID string) (*model.BoardHealth, error) {
	if _, err := s.repo.GetBoardByIDAndOrg(ctx, boardID, orgID); err != nil {
		return nil, err
	}

	overdue, atRisk, err := s.repo.GetTaskRiskCounts(ctx, boardID)
	if err != nil {
		return nil, err
	}

	const bottlenecks = 0 // requires stage-change timestamps we don't track yet

	status := "healthy"
	switch {
	case overdue > 5:
		status = "critical"
	case overdue+bottlenecks+atRisk > 0:
		status = "warning"
	}

	return &model.BoardHealth{
		Status:       status,
		OverdueTasks: overdue,
		Bottlenecks:  bottlenecks,
		AtRisk:       atRisk,
	}, nil
}

// GetBoardMembers returns all active members of the board's org with mapped roles.
// Verifies org scoping: returns errs.NotFound if board is not in the caller's org.
func (s *BoardService) GetBoardMembers(ctx context.Context, boardID, orgID string) ([]model.BoardMember, error) {
	return s.repo.GetBoardMembers(ctx, boardID, orgID)
}

// GetBoardActivity verifies the board belongs to the caller's org and returns
// its most recent activity entries (board events + events on its tasks).
func (s *BoardService) GetBoardActivity(ctx context.Context, boardID, orgID string, limit int) ([]model.BoardActivityEntry, error) {
	if _, err := s.repo.GetBoardByIDAndOrg(ctx, boardID, orgID); err != nil {
		return nil, err
	}

	if limit <= 0 {
		limit = 10
	}
	if limit > 50 {
		limit = 50
	}

	return s.repo.GetRecentActivityByBoard(ctx, boardID, limit)
}
