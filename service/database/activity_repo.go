package database

import (
	"context"
	"encoding/json"

	"github.com/jackc/pgx/v5/pgxpool"

	"syncspace-backend/errs"
	"syncspace-backend/model"
)

// ActivityRepo handles database operations for the activity_logs table.
type ActivityRepo struct {
	db *pgxpool.Pool
}

// NewActivityRepo creates an ActivityRepo with the provided connection pool.
func NewActivityRepo(db *pgxpool.Pool) *ActivityRepo {
	return &ActivityRepo{db: db}
}

// ActivityFilter holds optional query filters for listing activity logs.
// A zero-value (empty string) field means "no filter on this column".
type ActivityFilter struct {
	BoardID string // filter by entity_id when entity_type is 'board' or by task's board
	UserID  string // filter by actor_id
}

// InsertActivityLog persists a new activity log entry.
// Metadata is marshalled to JSONB. Called by ActivityLogger — never call directly.
func (r *ActivityRepo) InsertActivityLog(ctx context.Context, al *model.ActivityLog) error {
	meta, err := json.Marshal(al.Metadata)
	if err != nil {
		meta = []byte("{}")
	}

	_, err = r.db.Exec(ctx,
		`INSERT INTO activity_logs
		     (id, entity_type, entity_id, actor_id, action, metadata, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		al.ID, al.EntityType, al.EntityID, al.ActorID, al.Action, meta, al.CreatedAt,
	)
	if err != nil {
		return errs.Internal("failed to insert activity log")
	}
	return nil
}

// GetFiltered returns a paginated slice of activity logs with optional filters
// and the global total count across all pages.
//
// Filter behaviour:
//   - BoardID non-empty → entity_id matches the board OR the log is for a task
//     that belongs to that board (via subquery). This covers both direct board
//     events and task-level events within the board.
//   - UserID non-empty → actor_id must match (who performed the action).
//   - Both may be combined; both may be empty (returns everything visible).
//
// The total count is derived from COUNT(*) OVER () — a window function that
// evaluates before LIMIT/OFFSET, so no second COUNT query is needed.
func (r *ActivityRepo) GetFiltered(
	ctx context.Context,
	f ActivityFilter,
	limit, offset int,
) (*model.ActivityLogPage, error) {
	var boardArg, userArg *string
	if f.BoardID != "" {
		boardArg = &f.BoardID
	}
	if f.UserID != "" {
		userArg = &f.UserID
	}

	rows, err := r.db.Query(ctx,
		`SELECT
		     al.id,
		     al.entity_type,
		     al.entity_id,
		     al.actor_id,
		     al.action,
		     al.metadata,
		     al.created_at,
		     COALESCE(u.name, '') AS actor_name,
		     COUNT(*) OVER ()     AS total_count
		 FROM   activity_logs al
		 JOIN   users u ON u.id = al.actor_id
		 WHERE  (
		     $1::text IS NULL
		     OR al.entity_id = $1
		     OR (al.entity_type = 'task' AND al.entity_id IN (
		             SELECT id FROM tasks WHERE board_id = $1
		         ))
		 )
		   AND ($2::text IS NULL OR al.actor_id = $2)
		 ORDER  BY al.created_at DESC
		 LIMIT  $3
		 OFFSET $4`,
		boardArg, userArg, limit, offset,
	)
	if err != nil {
		return nil, errs.Internal("failed to query activity logs")
	}
	defer rows.Close()

	page := &model.ActivityLogPage{Logs: []model.ActivityLogDetail{}}

	for rows.Next() {
		var d model.ActivityLogDetail
		var metaBytes []byte
		if err := rows.Scan(
			&d.ID, &d.EntityType, &d.EntityID,
			&d.ActorID, &d.Action, &metaBytes, &d.CreatedAt,
			&d.ActorName,
			&page.Total,
		); err != nil {
			return nil, errs.Internal("failed to scan activity log row")
		}
		if err := json.Unmarshal(metaBytes, &d.Metadata); err != nil {
			d.Metadata = map[string]any{}
		}
		page.Logs = append(page.Logs, d)
	}
	if rows.Err() != nil {
		return nil, errs.Internal("activity log query error")
	}
	return page, nil
}
