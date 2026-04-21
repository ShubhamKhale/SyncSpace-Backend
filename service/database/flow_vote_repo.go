package database

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"

	"syncspace-backend/errs"
	"syncspace-backend/model"
)

// FlowVoteRepo handles all database operations for flow votes.
type FlowVoteRepo struct {
	db *pgxpool.Pool
}

// NewFlowVoteRepo creates a FlowVoteRepo with the provided connection pool.
func NewFlowVoteRepo(db *pgxpool.Pool) *FlowVoteRepo {
	return &FlowVoteRepo{db: db}
}

// UpsertVote inserts a vote or updates the vote_type when the user has already voted.
// Returns the persisted vote with server-assigned timestamps.
func (r *FlowVoteRepo) UpsertVote(ctx context.Context, flowID, userID, voteType string) (*model.FlowVote, error) {
	v := &model.FlowVote{}
	err := r.db.QueryRow(ctx,
		`INSERT INTO flow_votes (flow_id, user_id, vote_type, created_at, updated_at)
		 VALUES ($1, $2, $3, NOW(), NOW())
		 ON CONFLICT (flow_id, user_id) DO UPDATE
		     SET vote_type = EXCLUDED.vote_type,
		         updated_at = NOW()
		 RETURNING flow_id, user_id, vote_type, created_at, updated_at`,
		flowID, userID, voteType,
	).Scan(&v.FlowID, &v.UserID, &v.VoteType, &v.CreatedAt, &v.UpdatedAt)
	if err != nil {
		return nil, errs.Internal("failed to upsert vote")
	}
	return v, nil
}

// GetSummary returns up/down counts and the caller's current vote for a flow.
// The query always returns one row — COUNT never returns NULL on an empty set,
// and COALESCE handles a caller who has not yet voted (MAX → NULL → "").
func (r *FlowVoteRepo) GetSummary(ctx context.Context, flowID, callerID string) (*model.FlowVoteSummary, error) {
	s := &model.FlowVoteSummary{}
	err := r.db.QueryRow(ctx,
		`SELECT
		     COUNT(*) FILTER (WHERE vote_type = 'up')::int,
		     COUNT(*) FILTER (WHERE vote_type = 'down')::int,
		     COALESCE(MAX(vote_type) FILTER (WHERE user_id = $2), '')
		 FROM flow_votes
		 WHERE flow_id = $1`,
		flowID, callerID,
	).Scan(&s.Up, &s.Down, &s.MyVote)
	if err != nil {
		return nil, errs.Internal("failed to fetch vote summary")
	}
	return s, nil
}
