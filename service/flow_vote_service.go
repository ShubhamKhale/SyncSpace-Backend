package service

import (
	"context"

	"syncspace-backend/errs"
	"syncspace-backend/model"
	"syncspace-backend/service/database"
)

var validVoteTypes = map[string]bool{
	"up":   true,
	"down": true,
}

// FlowVoteService handles business logic for flow votes.
type FlowVoteService struct {
	repo     *database.FlowVoteRepo
	flowRepo *database.FlowRepo // used to verify the flow exists before voting
}

// NewFlowVoteService creates a FlowVoteService backed by the given repos.
func NewFlowVoteService(repo *database.FlowVoteRepo, flowRepo *database.FlowRepo) *FlowVoteService {
	return &FlowVoteService{repo: repo, flowRepo: flowRepo}
}

// CastVote records or updates the caller's vote on a flow.
// vote_type must be "up" or "down". Calling again with a different type
// replaces the previous vote (one vote per user per flow).
func (s *FlowVoteService) CastVote(ctx context.Context, flowID, callerID, voteType string) (*model.FlowVote, error) {
	if !validVoteTypes[voteType] {
		return nil, errs.BadRequest("vote_type must be 'up' or 'down'")
	}

	// Guard: return 404 if the flow doesn't exist.
	if _, err := s.flowRepo.GetFlowByID(ctx, flowID); err != nil {
		return nil, err
	}

	return s.repo.UpsertVote(ctx, flowID, callerID, voteType)
}

// GetSummary returns the vote totals and the caller's current vote for a flow.
func (s *FlowVoteService) GetSummary(ctx context.Context, flowID, callerID string) (*model.FlowVoteSummary, error) {
	// Guard: return 404 if the flow doesn't exist.
	if _, err := s.flowRepo.GetFlowByID(ctx, flowID); err != nil {
		return nil, err
	}

	return s.repo.GetSummary(ctx, flowID, callerID)
}
