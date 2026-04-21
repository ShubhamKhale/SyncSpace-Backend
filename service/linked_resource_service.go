package service

import (
	"context"
	"strconv"
	"time"

	"syncspace-backend/errs"
	"syncspace-backend/model"
	"syncspace-backend/pkg/utils"
	"syncspace-backend/service/database"
)

// LinkedResourceInput is the per-item DTO accepted by ReplaceAll.
type LinkedResourceInput struct {
	Label string // optional display name
	URL   string // required; must be non-empty
}

// LinkedResourceService handles board-level linked resource operations.
type LinkedResourceService struct {
	repo      *database.LinkedResourceRepo
	boardRepo *database.BoardRepo
	logger    *ActivityLogger
}

// NewLinkedResourceService creates a LinkedResourceService.
func NewLinkedResourceService(
	repo *database.LinkedResourceRepo,
	boardRepo *database.BoardRepo,
	logger *ActivityLogger,
) *LinkedResourceService {
	return &LinkedResourceService{repo: repo, boardRepo: boardRepo, logger: logger}
}

// GetByBoard returns all linked resources for the board.
// Returns errs.NotFound when the board does not exist.
func (s *LinkedResourceService) GetByBoard(ctx context.Context, boardID string) ([]model.LinkedResource, error) {
	if _, err := s.boardRepo.GetBoardByID(ctx, boardID); err != nil {
		return nil, err // propagates NotFound / Internal as-is
	}
	return s.repo.GetByBoard(ctx, boardID)
}

// ReplaceAll atomically replaces the full list of linked resources for a board.
//
// Validation rules (applied before any DB write):
//   - Each item must have a non-empty URL.
//   - Duplicate URLs within the same request are rejected.
//   - An empty items slice is valid — it clears all resources.
//
// Returns the newly persisted slice (with server-assigned IDs and timestamps).
func (s *LinkedResourceService) ReplaceAll(
	ctx context.Context,
	boardID, callerID string,
	items []LinkedResourceInput,
) ([]model.LinkedResource, error) {
	// Board must exist before we proceed.
	if _, err := s.boardRepo.GetBoardByID(ctx, boardID); err != nil {
		return nil, err
	}

	// ── Validate items ────────────────────────────────────────────────────────
	seen := make(map[string]bool, len(items))
	for i, item := range items {
		if item.URL == "" {
			return nil, errs.BadRequest("item at index " + strconv.Itoa(i) + " is missing a url")
		}
		if seen[item.URL] {
			return nil, errs.BadRequest("duplicate url: " + item.URL)
		}
		seen[item.URL] = true
	}

	// ── Build models ──────────────────────────────────────────────────────────
	now := time.Now()
	resources := make([]model.LinkedResource, len(items))
	for i, item := range items {
		resources[i] = model.LinkedResource{
			ID:        utils.NewUUID(),
			BoardID:   boardID,
			Label:     item.Label,
			URL:       item.URL,
			CreatedBy: callerID,
			CreatedAt: now,
			UpdatedAt: now,
		}
	}

	if err := s.repo.ReplaceAll(ctx, boardID, resources); err != nil {
		return nil, err
	}

	s.logger.LogBoard(ctx, callerID, boardID, ActionUpdated, map[string]any{
		"field":         "linked_resources",
		"resource_count": len(resources),
	})

	return resources, nil
}

