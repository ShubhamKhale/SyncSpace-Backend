package service

import (
	"context"
	"time"

	"syncspace-backend/errs"
	"syncspace-backend/model"
	"syncspace-backend/pkg/utils"
	"syncspace-backend/service/database"
)

// BoardFlowService orchestrates board-scoped flow diagram use cases.
type BoardFlowService struct {
	repo      *database.BoardFlowRepo
	boardRepo *database.BoardRepo
}

// NewBoardFlowService creates a BoardFlowService wired to the provided repositories.
func NewBoardFlowService(repo *database.BoardFlowRepo, boardRepo *database.BoardRepo) *BoardFlowService {
	return &BoardFlowService{repo: repo, boardRepo: boardRepo}
}

func (s *BoardFlowService) verifyBoard(ctx context.Context, boardID, orgID string) error {
	_, err := s.boardRepo.GetBoardByIDAndOrg(ctx, boardID, orgID)
	return err
}

func denyViewer(orgRole string) error {
	if orgRole == "viewer" {
		return errs.Forbidden("viewers cannot modify flows")
	}
	return nil
}

// ListFlows returns all flow metadata for a board, newest-updated first.
func (s *BoardFlowService) ListFlows(ctx context.Context, boardID, orgID string) ([]model.BoardFlow, error) {
	if err := s.verifyBoard(ctx, boardID, orgID); err != nil {
		return nil, err
	}
	return s.repo.ListByBoard(ctx, boardID)
}

// CreateFlow creates a new empty flow on the board.
func (s *BoardFlowService) CreateFlow(ctx context.Context, boardID, orgID, orgRole, name string) (*model.BoardFlow, error) {
	if err := s.verifyBoard(ctx, boardID, orgID); err != nil {
		return nil, err
	}
	if err := denyViewer(orgRole); err != nil {
		return nil, err
	}
	if name == "" {
		name = "Untitled Flow 1"
	}
	now := time.Now()
	f := &model.BoardFlow{
		ID:        utils.NewUUID(),
		BoardID:   boardID,
		Name:      name,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := s.repo.Create(ctx, f); err != nil {
		return nil, err
	}
	return f, nil
}

// GetFlow returns flow metadata.
func (s *BoardFlowService) GetFlow(ctx context.Context, boardID, orgID, flowID string) (*model.BoardFlow, error) {
	if err := s.verifyBoard(ctx, boardID, orgID); err != nil {
		return nil, err
	}
	return s.repo.GetByID(ctx, boardID, flowID)
}

// RenameFlow updates the flow name.
func (s *BoardFlowService) RenameFlow(ctx context.Context, boardID, orgID, orgRole, flowID, name string) (*model.BoardFlow, error) {
	if err := s.verifyBoard(ctx, boardID, orgID); err != nil {
		return nil, err
	}
	if err := denyViewer(orgRole); err != nil {
		return nil, err
	}
	if name == "" {
		return nil, errs.BadRequest("name is required")
	}
	return s.repo.Rename(ctx, boardID, flowID, name)
}

// DeleteFlow permanently removes a flow and its diagram data.
func (s *BoardFlowService) DeleteFlow(ctx context.Context, boardID, orgID, orgRole, flowID string) error {
	if err := s.verifyBoard(ctx, boardID, orgID); err != nil {
		return err
	}
	if err := denyViewer(orgRole); err != nil {
		return err
	}
	return s.repo.Delete(ctx, boardID, flowID)
}

// DuplicateFlow deep-copies a flow and its diagram. Name defaults to "<source name> (Copy)".
func (s *BoardFlowService) DuplicateFlow(ctx context.Context, boardID, orgID, orgRole, flowID, name string) (*model.BoardFlow, error) {
	if err := s.verifyBoard(ctx, boardID, orgID); err != nil {
		return nil, err
	}
	if err := denyViewer(orgRole); err != nil {
		return nil, err
	}
	if name == "" {
		src, err := s.repo.GetByID(ctx, boardID, flowID)
		if err != nil {
			return nil, err
		}
		name = src.Name + " (Copy)"
	}
	return s.repo.Duplicate(ctx, boardID, flowID, utils.NewUUID(), name)
}

// GetDiagram returns the canvas state for a flow (empty default if never saved).
func (s *BoardFlowService) GetDiagram(ctx context.Context, boardID, orgID, flowID string) (*model.DiagramData, error) {
	if err := s.verifyBoard(ctx, boardID, orgID); err != nil {
		return nil, err
	}
	// Verify flow belongs to this board.
	if _, err := s.repo.GetByID(ctx, boardID, flowID); err != nil {
		return nil, err
	}
	return s.repo.GetDiagram(ctx, flowID)
}

// SaveDiagram upserts diagram data and updates node/edge counts on the flow row.
func (s *BoardFlowService) SaveDiagram(ctx context.Context, boardID, orgID, orgRole, flowID string, d *model.DiagramData) (time.Time, int, int, error) {
	if err := s.verifyBoard(ctx, boardID, orgID); err != nil {
		return time.Time{}, 0, 0, err
	}
	if err := denyViewer(orgRole); err != nil {
		return time.Time{}, 0, 0, err
	}
	if _, err := s.repo.GetByID(ctx, boardID, flowID); err != nil {
		return time.Time{}, 0, 0, err
	}
	return s.repo.UpsertDiagram(ctx, flowID, d)
}
