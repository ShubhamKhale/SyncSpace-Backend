package service

import (
	"context"
	"encoding/json"

	"syncspace-backend/service/database"
)

// FlowAIConversationService manages per-user, per-flow AI conversation persistence.
type FlowAIConversationService struct {
	repo *database.FlowAIConversationRepo
}

// NewFlowAIConversationService creates a FlowAIConversationService.
func NewFlowAIConversationService(repo *database.FlowAIConversationRepo) *FlowAIConversationService {
	return &FlowAIConversationService{repo: repo}
}

// Get returns the stored conversation for (userID, flowID), or nil if none exists.
func (s *FlowAIConversationService) Get(ctx context.Context, userID, flowID string) (*database.FlowAIConversation, error) {
	return s.repo.Get(ctx, userID, flowID)
}

// Save persists the conversation for (userID, flowID).
// history and messages are arbitrary JSON arrays passed through as-is.
func (s *FlowAIConversationService) Save(ctx context.Context, userID, flowID string, history, messages json.RawMessage) error {
	return s.repo.Upsert(ctx, userID, flowID, history, messages)
}
