package database

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"

	"syncspace-backend/errs"
	"syncspace-backend/model"
)

// DashboardRepo provides aggregated queries for the dashboard overview.
type DashboardRepo struct {
	db *pgxpool.Pool
}

// NewDashboardRepo creates a DashboardRepo with the provided connection pool.
func NewDashboardRepo(db *pgxpool.Pool) *DashboardRepo {
	return &DashboardRepo{db: db}
}

// GetStats returns aggregated task and board counts for a given user.
// A single CTE + COUNT FILTER query avoids N+1 lookups.
//
// board_count   — boards owned by the user
// task_*        — tasks across those boards, bucketed by stage
// task_due_soon — not-done tasks whose due_date falls within the next 7 days
func (r *DashboardRepo) GetStats(ctx context.Context, userID string) (*model.DashboardStats, error) {
	s := &model.DashboardStats{}
	err := r.db.QueryRow(ctx,
		`WITH user_boards AS (
		     SELECT id FROM boards WHERE owner_id = $1
		 )
		 SELECT
		     (SELECT COUNT(*)::int FROM user_boards)                                AS board_count,
		     COUNT(*) FILTER (WHERE stage = 'todo')::int                            AS task_todo,
		     COUNT(*) FILTER (WHERE stage = 'in_progress')::int                     AS task_in_progress,
		     COUNT(*) FILTER (WHERE stage = 'done')::int                            AS task_done,
		     COUNT(*) FILTER (
		         WHERE stage    != 'done'
		           AND due_date IS NOT NULL
		           AND due_date BETWEEN NOW() AND NOW() + INTERVAL '7 days'
		     )::int                                                                  AS task_due_soon
		 FROM tasks
		 WHERE board_id IN (SELECT id FROM user_boards)`,
		userID,
	).Scan(
		&s.BoardCount,
		&s.TaskTodo,
		&s.TaskInProgress,
		&s.TaskDone,
		&s.TaskDueSoon,
	)
	if err != nil {
		return nil, errs.Internal("failed to query dashboard stats")
	}
	return s, nil
}
