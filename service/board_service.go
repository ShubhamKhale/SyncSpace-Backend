// Package service contains the application's business-logic layer.
// Services coordinate between the operation (validation/transformation) layer
// and the database (persistence) layer. They must never be called directly
// from the router — only through controllers.
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
// ownerID must be the authenticated user's ID (set by JWT middleware).
func (s *BoardService) CreateBoard(ctx context.Context, ownerID, title, description string) (*model.Board, error) {
	if err := operation.ValidateCreateBoard(title, description); err != nil {
		return nil, err
	}

	now := time.Now()
	board := &model.Board{
		ID:          utils.NewUUID(),
		Title:       title,
		Description: description,
		OwnerID:     ownerID,
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

// GetBoardsByOwner retrieves all boards owned by the given user.
func (s *BoardService) GetBoardsByOwner(ctx context.Context, ownerID string) ([]model.Board, error) {
	if ownerID == "" {
		return nil, errs.BadRequest("owner ID is required")
	}
	return s.repo.GetBoardsByOwner(ctx, ownerID)
}

// GetRecentBoards returns up to limit boards for the user, sorted by last update.
// limit is clamped to [1, 20].
func (s *BoardService) GetRecentBoards(ctx context.Context, ownerID string, limit int) ([]model.Board, error) {
	if limit <= 0 {
		limit = 5
	}
	if limit > 20 {
		limit = 20
	}
	return s.repo.GetRecentBoardsByOwner(ctx, ownerID, limit)
}
