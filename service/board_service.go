// Package service contains the application's business-logic layer.
// Services coordinate between the operation (validation/transformation) layer
// and the database (persistence) layer. They must never be called directly
// from the router — only through controllers.
package service

import (
	"context"
	"fmt"
	"time"

	"syncspace-backend/model"
	"syncspace-backend/service/database"
	"syncspace-backend/service/operation"
)

// BoardService orchestrates board-related use cases.
type BoardService struct {
	repo *database.BoardRepo
}

// NewBoardService creates a BoardService wired to the provided repository.
func NewBoardService(repo *database.BoardRepo) *BoardService {
	return &BoardService{repo: repo}
}

// CreateBoard validates the input, builds a Board model, and persists it.
func (s *BoardService) CreateBoard(ctx context.Context, title, description string) (*model.Board, error) {
	if err := operation.ValidateCreateBoard(title, description); err != nil {
		return nil, err
	}

	board := &model.Board{
		ID:          fmt.Sprintf("board-%d", time.Now().UnixNano()),
		Title:       title,
		Description: description,
		CreatedAt:   time.Now(),
	}

	if err := s.repo.InsertBoard(ctx, board); err != nil {
		return nil, err
	}

	return board, nil
}

// GetBoards retrieves all boards for the workspace.
func (s *BoardService) GetBoards(ctx context.Context) ([]model.Board, error) {
	return s.repo.GetBoards(ctx)
}
