package database

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"syncspace-backend/errs"
)

// FlowAIConversationRepo persists per-user, per-flow AI conversation state.
type FlowAIConversationRepo struct {
	db *pgxpool.Pool
}

// NewFlowAIConversationRepo creates a FlowAIConversationRepo backed by the given pool.
func NewFlowAIConversationRepo(db *pgxpool.Pool) *FlowAIConversationRepo {
	return &FlowAIConversationRepo{db: db}
}

// FlowAIConversation holds the stored conversation for one user + flow pair.
type FlowAIConversation struct {
	History  json.RawMessage `json:"history"`
	Messages json.RawMessage `json:"messages"`
}

// Get returns the stored conversation for (userID, flowID).
// Returns (nil, nil) when no row exists yet.
func (r *FlowAIConversationRepo) Get(ctx context.Context, userID, flowID string) (*FlowAIConversation, error) {
	var c FlowAIConversation
	err := r.db.QueryRow(ctx,
		`SELECT history, messages
		 FROM public.flow_ai_conversations
		 WHERE user_id = $1 AND flow_id = $2`,
		userID, flowID,
	).Scan(&c.History, &c.Messages)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, errs.Internal("failed to fetch ai conversation")
	}
	return &c, nil
}

// Upsert inserts or replaces the conversation for (userID, flowID).
func (r *FlowAIConversationRepo) Upsert(ctx context.Context, userID, flowID string, history, messages json.RawMessage) error {
	_, err := r.db.Exec(ctx,
		`INSERT INTO public.flow_ai_conversations (user_id, flow_id, history, messages, updated_at)
		 VALUES ($1, $2, $3, $4, NOW())
		 ON CONFLICT (user_id, flow_id)
		 DO UPDATE SET history = EXCLUDED.history, messages = EXCLUDED.messages, updated_at = NOW()`,
		userID, flowID, history, messages,
	)
	if err != nil {
		return errs.Internal("failed to save ai conversation")
	}
	return nil
}
