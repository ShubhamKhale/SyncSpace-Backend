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

// GetStats returns aggregated task and board counts for a given org.
// A single CTE + COUNT FILTER query avoids N+1 lookups.
//
// board_count   — boards in the org
// task_*        — tasks across those boards, bucketed by stage
// task_due_soon — not-done tasks whose due_date falls within the next 7 days
func (r *DashboardRepo) GetStats(ctx context.Context, orgID string) (*model.DashboardStats, error) {
	s := &model.DashboardStats{}
	err := r.db.QueryRow(ctx,
		`WITH org_boards AS (
		     SELECT id FROM public.boards WHERE org_id = $1
		 )
		 SELECT
		     (SELECT COUNT(*)::int FROM org_boards)                                 AS board_count,
		     COUNT(*) FILTER (WHERE stage = 'todo')::int                            AS task_todo,
		     COUNT(*) FILTER (WHERE stage = 'in_progress')::int                     AS task_in_progress,
		     COUNT(*) FILTER (WHERE stage = 'done')::int                            AS task_done,
		     COUNT(*) FILTER (
		         WHERE stage    != 'done'
		           AND due_date IS NOT NULL
		           AND due_date BETWEEN NOW() AND NOW() + INTERVAL '7 days'
		     )::int                                                                  AS task_due_soon,
		     (SELECT COUNT(*)::int FROM public.organization_members
		      WHERE  organization_id = $1 AND status = 'active')                    AS team_members
		 FROM public.tasks
		 WHERE board_id IN (SELECT id FROM org_boards)`,
		orgID,
	).Scan(
		&s.BoardCount,
		&s.TaskTodo,
		&s.TaskInProgress,
		&s.TaskDone,
		&s.TaskDueSoon,
		&s.TeamMembers,
	)
	if err != nil {
		return nil, errs.Internal("failed to query dashboard stats")
	}
	return s, nil
}
