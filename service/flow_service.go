package service

import (
	"context"
	"encoding/json"
	"time"

	"syncspace-backend/errs"
	"syncspace-backend/model"
	"syncspace-backend/service/database"
)

// FlowService handles business logic for visual flow diagrams.
type FlowService struct {
	repo *database.FlowRepo
}

// NewFlowService creates a FlowService backed by the provided repo.
func NewFlowService(repo *database.FlowRepo) *FlowService {
	return &FlowService{repo: repo}
}

// GetFlow returns the flow with the given ID.
func (s *FlowService) GetFlow(ctx context.Context, flowID string) (*model.Flow, error) {
	return s.repo.GetFlowByID(ctx, flowID)
}

// UpdateFlow replaces the flow's data payload and increments its version.
//
// expectedVersion must match the current DB version (optimistic locking).
// A mismatch returns errs.Conflict so the caller can re-fetch, merge, and retry.
//
// data must be a valid JSON object — e.g. {"nodes":[...],"edges":[...]}.
func (s *FlowService) UpdateFlow(
	ctx context.Context,
	flowID, callerID string,
	rawData json.RawMessage,
	expectedVersion int,
) (*model.Flow, error) {
	// Validate that data is a JSON object, not null or an array.
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(rawData, &obj); err != nil {
		return nil, errs.BadRequest("data must be a JSON object (e.g. {\"nodes\":[],\"edges\":[]})")
	}

	f := &model.Flow{
		ID:             flowID,
		Data:           rawData,
		LastModifiedBy: callerID,
		UpdatedAt:      time.Now().UTC(),
	}

	if err := s.repo.UpdateFlow(ctx, f, expectedVersion); err != nil {
		return nil, err
	}

	// Re-fetch to return the full updated row (includes incremented version, board_id, title, etc.).
	return s.repo.GetFlowByID(ctx, flowID)
}
