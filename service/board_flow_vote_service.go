package service

import (
	"context"

	"syncspace-backend/errs"
	"syncspace-backend/model"
	"syncspace-backend/service/database"
)

var validBoardFlowVotes = map[string]bool{
	"approve": true,
	"review":  true,
	"reject":  true,
}

// BoardFlowVoteService orchestrates vote operations for board flow diagrams.
type BoardFlowVoteService struct {
	repo      *database.BoardFlowVoteRepo
	boardRepo *database.BoardRepo
}

// NewBoardFlowVoteService creates a BoardFlowVoteService wired to the provided repos.
func NewBoardFlowVoteService(repo *database.BoardFlowVoteRepo, boardRepo *database.BoardRepo) *BoardFlowVoteService {
	return &BoardFlowVoteService{repo: repo, boardRepo: boardRepo}
}

func (s *BoardFlowVoteService) verifyBoard(ctx context.Context, boardID, orgID string) error {
	_, err := s.boardRepo.GetBoardByIDAndOrg(ctx, boardID, orgID)
	return err
}

// GetVotes returns all votes for a flow.
func (s *BoardFlowVoteService) GetVotes(ctx context.Context, boardID, orgID, flowID string) ([]model.BoardFlowVote, error) {
	if err := s.verifyBoard(ctx, boardID, orgID); err != nil {
		return nil, err
	}
	return s.repo.List(ctx, boardID, flowID)
}

// CastVote upserts the caller's vote. Returns the persisted vote with user name.
func (s *BoardFlowVoteService) CastVote(ctx context.Context, boardID, orgID, flowID, userID, vote string) (*model.BoardFlowVote, error) {
	if !validBoardFlowVotes[vote] {
		return nil, errs.BadRequest("vote must be approve, review, or reject")
	}
	if err := s.verifyBoard(ctx, boardID, orgID); err != nil {
		return nil, err
	}
	return s.repo.Upsert(ctx, boardID, flowID, userID, vote)
}

// RemoveVote deletes the caller's vote. Idempotent — no error if vote doesn't exist.
func (s *BoardFlowVoteService) RemoveVote(ctx context.Context, boardID, orgID, flowID, userID string) error {
	if err := s.verifyBoard(ctx, boardID, orgID); err != nil {
		return err
	}
	return s.repo.Delete(ctx, boardID, flowID, userID)
}
