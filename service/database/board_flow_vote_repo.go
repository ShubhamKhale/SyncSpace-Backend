package database

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"

	"syncspace-backend/errs"
	"syncspace-backend/model"
)

// BoardFlowVoteRepo handles database operations for board_flow_votes.
type BoardFlowVoteRepo struct {
	db *pgxpool.Pool
}

// NewBoardFlowVoteRepo creates a BoardFlowVoteRepo with the provided pool.
func NewBoardFlowVoteRepo(db *pgxpool.Pool) *BoardFlowVoteRepo {
	return &BoardFlowVoteRepo{db: db}
}

// Upsert inserts or updates the user's vote, returning the full vote with user name.
func (r *BoardFlowVoteRepo) Upsert(ctx context.Context, boardID, flowID, userID, vote string) (*model.BoardFlowVote, error) {
	v := &model.BoardFlowVote{}
	err := r.db.QueryRow(ctx,
		`WITH upserted AS (
		     INSERT INTO public.board_flow_votes (board_id, flow_id, user_id, vote, created_at, updated_at)
		     VALUES ($1, $2, $3, $4, NOW(), NOW())
		     ON CONFLICT (board_id, flow_id, user_id) DO UPDATE
		         SET vote = EXCLUDED.vote, updated_at = NOW()
		     RETURNING user_id, vote
		 )
		 SELECT u.id, u.name, upserted.vote
		 FROM upserted
		 JOIN public.users u ON u.id = upserted.user_id`,
		boardID, flowID, userID, vote,
	).Scan(&v.UserID, &v.Name, &v.Vote)
	if err != nil {
		return nil, errs.Internal("failed to upsert vote")
	}
	return v, nil
}

// List returns all votes for a flow, each with the voter's name.
func (r *BoardFlowVoteRepo) List(ctx context.Context, boardID, flowID string) ([]model.BoardFlowVote, error) {
	rows, err := r.db.Query(ctx,
		`SELECT v.user_id, u.name, v.vote
		 FROM public.board_flow_votes v
		 JOIN public.users u ON u.id = v.user_id
		 WHERE v.board_id = $1 AND v.flow_id = $2
		 ORDER BY v.created_at`,
		boardID, flowID,
	)
	if err != nil {
		return nil, errs.Internal("failed to list votes")
	}
	defer rows.Close()

	var votes []model.BoardFlowVote
	for rows.Next() {
		var v model.BoardFlowVote
		if err := rows.Scan(&v.UserID, &v.Name, &v.Vote); err != nil {
			return nil, errs.Internal("failed to scan vote")
		}
		votes = append(votes, v)
	}
	if votes == nil {
		votes = []model.BoardFlowVote{}
	}
	return votes, nil
}

// Delete removes the user's vote. Not an error if no row exists.
func (r *BoardFlowVoteRepo) Delete(ctx context.Context, boardID, flowID, userID string) error {
	_, err := r.db.Exec(ctx,
		`DELETE FROM public.board_flow_votes WHERE board_id = $1 AND flow_id = $2 AND user_id = $3`,
		boardID, flowID, userID,
	)
	if err != nil {
		return errs.Internal("failed to delete vote")
	}
	return nil
}
