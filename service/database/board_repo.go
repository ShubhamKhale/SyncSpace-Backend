// Package database contains repository types that interact directly with
// the PostgreSQL database. Each repository receives a *pgxpool.Pool via
// constructor injection and exposes methods that accept a context.Context.
package database

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"syncspace-backend/errs"
	"syncspace-backend/model"
)

// BoardRepo handles all database operations for the Board entity.
type BoardRepo struct {
	db *pgxpool.Pool
}

// NewBoardRepo creates a new BoardRepo with the provided connection pool.
func NewBoardRepo(db *pgxpool.Pool) *BoardRepo {
	return &BoardRepo{db: db}
}

// scanBoard reads one board row in the standard column order (includes org_id).
func scanBoard(row pgx.Row) (*model.Board, error) {
	b := &model.Board{}
	err := row.Scan(&b.ID, &b.Title, &b.Description, &b.OwnerID, &b.OrgID, &b.CreatedAt, &b.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errs.NotFound("board not found")
		}
		return nil, errs.Internal("failed to scan board")
	}
	return b, nil
}

// InsertBoard persists a new board record.
func (r *BoardRepo) InsertBoard(ctx context.Context, board *model.Board) error {
	_, err := r.db.Exec(ctx,
		`INSERT INTO public.boards (id, title, description, owner_id, org_id, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		board.ID, board.Title, board.Description, board.OwnerID, board.OrgID, board.CreatedAt, board.UpdatedAt,
	)
	if err != nil {
		return errs.Internal("failed to insert board")
	}
	return nil
}

// GetBoardByID returns a single board, or errs.NotFound.
func (r *BoardRepo) GetBoardByID(ctx context.Context, id string) (*model.Board, error) {
	row := r.db.QueryRow(ctx,
		`SELECT id, title, description, owner_id, org_id, created_at, updated_at
		 FROM public.boards WHERE id = $1`,
		id,
	)
	return scanBoard(row)
}

// GetBoardByIDAndOrg returns a board only if it belongs to the given org.
func (r *BoardRepo) GetBoardByIDAndOrg(ctx context.Context, boardID, orgID string) (*model.Board, error) {
	row := r.db.QueryRow(ctx,
		`SELECT id, title, description, owner_id, org_id, created_at, updated_at
		 FROM public.boards WHERE id = $1 AND org_id = $2`,
		boardID, orgID,
	)
	return scanBoard(row)
}

// GetBoardsByOrg returns all boards for the given org, newest first.
func (r *BoardRepo) GetBoardsByOrg(ctx context.Context, orgID string) ([]model.Board, error) {
	rows, err := r.db.Query(ctx,
		`SELECT id, title, description, owner_id, org_id, created_at, updated_at
		 FROM   public.boards
		 WHERE  org_id = $1
		 ORDER  BY created_at DESC`,
		orgID,
	)
	if err != nil {
		return nil, errs.Internal("failed to query boards")
	}
	defer rows.Close()

	return collectBoards(rows)
}

// GetRecentBoardsByOrg returns the most recently updated boards for the org.
func (r *BoardRepo) GetRecentBoardsByOrg(ctx context.Context, orgID string, limit int) ([]model.Board, error) {
	rows, err := r.db.Query(ctx,
		`SELECT id, title, description, owner_id, org_id, created_at, updated_at
		 FROM   public.boards
		 WHERE  org_id = $1
		 ORDER  BY updated_at DESC
		 LIMIT  $2`,
		orgID, limit,
	)
	if err != nil {
		return nil, errs.Internal("failed to query recent boards")
	}
	defer rows.Close()

	return collectBoards(rows)
}

// GetBoardsByOwner returns all boards owned by the given user. Kept for internal use.
func (r *BoardRepo) GetBoardsByOwner(ctx context.Context, ownerID string) ([]model.Board, error) {
	rows, err := r.db.Query(ctx,
		`SELECT id, title, description, owner_id, org_id, created_at, updated_at
		 FROM   public.boards
		 WHERE  owner_id = $1
		 ORDER  BY created_at DESC`,
		ownerID,
	)
	if err != nil {
		return nil, errs.Internal("failed to query boards")
	}
	defer rows.Close()

	return collectBoards(rows)
}

// UpdateBoard applies partial updates to a board's title and/or description.
// Only non-nil pointer fields are written; all others are left unchanged via COALESCE.
// The board must belong to the given org (enforced in WHERE) — returns errs.NotFound otherwise.
func (r *BoardRepo) UpdateBoard(ctx context.Context, boardID, orgID string, title, description *string) (*model.Board, error) {
	row := r.db.QueryRow(ctx,
		`UPDATE public.boards
		 SET    title       = COALESCE($2, title),
		        description = COALESCE($3, description),
		        updated_at  = NOW()
		 WHERE  id = $1 AND org_id = $4
		 RETURNING id, title, description, owner_id, org_id, created_at, updated_at`,
		boardID, title, description, orgID,
	)
	return scanBoard(row)
}

// GetTaskRiskCounts returns the raw counts behind a board's health summary:
// overdue (past due, not done) and at-risk (high priority, due within 2 days, not done).
// Bottleneck detection requires stage-change timestamps we don't track yet,
// so it's left to the caller to report as zero.
func (r *BoardRepo) GetTaskRiskCounts(ctx context.Context, boardID string) (overdue, atRisk int, err error) {
	err = r.db.QueryRow(ctx,
		`SELECT
		     COUNT(*) FILTER (
		         WHERE due_date IS NOT NULL AND due_date < NOW() AND stage != 'done'
		     )::int AS overdue,
		     COUNT(*) FILTER (
		         WHERE priority = 'high' AND due_date IS NOT NULL
		           AND due_date >= NOW() AND due_date <= NOW() + INTERVAL '2 days'
		           AND stage != 'done'
		     )::int AS at_risk
		 FROM public.tasks
		 WHERE board_id = $1`,
		boardID,
	).Scan(&overdue, &atRisk)
	if err != nil {
		return 0, 0, errs.Internal("failed to query board task risk counts")
	}
	return overdue, atRisk, nil
}

// GetRecentActivityByBoard returns the most recent activity entries for a single
// board — both direct board events and events on the board's tasks — with the
// actor name, a humanised action phrase, and the affected entity's title resolved
// via JOINs (no extra round-trips, mirrors ActivityRepo.GetFiltered's join shape).
func (r *BoardRepo) GetRecentActivityByBoard(ctx context.Context, boardID string, limit int) ([]model.BoardActivityEntry, error) {
	rows, err := r.db.Query(ctx,
		`SELECT
		     al.id,
		     COALESCE(u.name, '') AS actor,
		     CASE
		         WHEN al.entity_type = 'task'  AND al.action = 'created'  THEN 'added a new task'
		         WHEN al.entity_type = 'task'  AND al.action = 'updated'  THEN 'updated a task'
		         WHEN al.entity_type = 'task'  AND al.action = 'moved'    THEN 'moved a task'
		         WHEN al.entity_type = 'task'  AND al.action = 'assigned' THEN 'assigned a task'
		         WHEN al.entity_type = 'task'  AND al.action = 'deleted'  THEN 'deleted a task'
		         WHEN al.entity_type = 'board' AND al.action = 'created'  THEN 'created the board'
		         WHEN al.entity_type = 'board' AND al.action = 'updated'  THEN 'updated the board'
		         ELSE al.action || ' ' || al.entity_type
		     END AS action,
		     COALESCE(t_ref.title, b_direct.title, '') AS target,
		     al.created_at
		 FROM   public.activity_logs al
		 JOIN   public.users u ON u.id = al.actor_id
		 LEFT JOIN public.boards b_direct
		        ON b_direct.id = al.entity_id AND al.entity_type = 'board'
		 LEFT JOIN public.tasks t_ref
		        ON t_ref.id = al.entity_id AND al.entity_type = 'task'
		 WHERE  b_direct.id = $1 OR t_ref.board_id = $1
		 ORDER  BY al.created_at DESC
		 LIMIT  $2`,
		boardID, limit,
	)
	if err != nil {
		return nil, errs.Internal("failed to query board activity")
	}
	defer rows.Close()

	entries := []model.BoardActivityEntry{}
	for rows.Next() {
		var e model.BoardActivityEntry
		if err := rows.Scan(&e.ID, &e.Actor, &e.Action, &e.Target, &e.CreatedAt); err != nil {
			return nil, errs.Internal("failed to scan board activity row")
		}
		entries = append(entries, e)
	}
	if rows.Err() != nil {
		return nil, errs.Internal("board activity query error")
	}
	return entries, nil
}

// GetBoardMembers returns all active org members for the board's org.
// The board owner gets role "owner"; org admins and members get "editor"; viewers get "viewer".
// Returns errs.NotFound if board does not exist in the given org.
func (r *BoardRepo) GetBoardMembers(ctx context.Context, boardID, orgID string) ([]model.BoardMember, error) {
	rows, err := r.db.Query(ctx,
		`SELECT
		     u.id,
		     COALESCE(u.name, '')       AS name,
		     u.email,
		     COALESCE(u.avatar_url, '') AS avatar_url,
		     CASE
		         WHEN b.owner_id = u.id       THEN 'owner'
		         WHEN om.role IN ('admin','member') THEN 'editor'
		         ELSE 'viewer'
		     END AS role
		 FROM   public.boards b
		 JOIN   public.organization_members om
		        ON om.organization_id = b.org_id AND om.status = 'active'
		 JOIN   public.users u ON u.id = om.user_id
		 WHERE  b.id = $1 AND b.org_id = $2
		 ORDER  BY
		     CASE WHEN b.owner_id = u.id THEN 0 ELSE 1 END,
		     u.name ASC`,
		boardID, orgID,
	)
	if err != nil {
		return nil, errs.Internal("failed to query board members")
	}
	defer rows.Close()

	members := []model.BoardMember{}
	for rows.Next() {
		var m model.BoardMember
		if err := rows.Scan(&m.ID, &m.Name, &m.Email, &m.AvatarURL, &m.Role); err != nil {
			return nil, errs.Internal("failed to scan board member row")
		}
		members = append(members, m)
	}
	if rows.Err() != nil {
		return nil, errs.Internal("board members query error")
	}

	// Empty result means board not found in this org (no members = board doesn't exist there).
	if len(members) == 0 {
		// Confirm board exists vs org mismatch.
		var exists bool
		_ = r.db.QueryRow(ctx,
			`SELECT EXISTS(SELECT 1 FROM public.boards WHERE id=$1 AND org_id=$2)`,
			boardID, orgID,
		).Scan(&exists)
		if !exists {
			return nil, errs.NotFound("board not found")
		}
	}

	return members, nil
}

// collectBoards scans all rows from a board query into a slice.
func collectBoards(rows pgx.Rows) ([]model.Board, error) {
	var boards []model.Board
	for rows.Next() {
		var b model.Board
		if err := rows.Scan(&b.ID, &b.Title, &b.Description, &b.OwnerID, &b.OrgID, &b.CreatedAt, &b.UpdatedAt); err != nil {
			return nil, errs.Internal("failed to scan board row")
		}
		boards = append(boards, b)
	}
	if rows.Err() != nil {
		return nil, errs.Internal("board query error")
	}
	return boards, nil
}
