package database

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"syncspace-backend/errs"
	"syncspace-backend/model"
)

// AnalyticsRepo provides read-only aggregation queries for the analytics
// endpoints. All queries are scoped to boards owned by the requesting user;
// an optional boardID further narrows results to a single board.
type AnalyticsRepo struct {
	db *pgxpool.Pool
}

// NewAnalyticsRepo creates an AnalyticsRepo with the provided connection pool.
func NewAnalyticsRepo(db *pgxpool.Pool) *AnalyticsRepo {
	return &AnalyticsRepo{db: db}
}

// ── Task completion trend ─────────────────────────────────────────────────────

// GetTaskCompletionTrend returns one row per calendar day from since to today.
// Each row carries the count of tasks created on that day and the count of tasks
// that reached the 'done' stage on that day (proxied by updated_at).
//
// generate_series fills every day so the caller always gets a continuous series
// with zeros on quiet days — no client-side gap-filling required.
func (r *AnalyticsRepo) GetTaskCompletionTrend(
	ctx context.Context,
	userID string,
	since time.Time,
	boardID *string,
) ([]model.TaskTrendPoint, error) {
	rows, err := r.db.Query(ctx,
		`WITH board_scope AS (
		     SELECT id FROM boards WHERE owner_id = $1
		 ),
		 date_series AS (
		     SELECT generate_series($2::timestamptz, NOW(), '1 day')::date AS day
		 ),
		 created_counts AS (
		     SELECT DATE_TRUNC('day', created_at)::date AS day,
		            COUNT(*)::int                        AS cnt
		     FROM   tasks
		     WHERE  board_id IN (SELECT id FROM board_scope)
		       AND  ($3::text IS NULL OR board_id = $3)
		       AND  created_at >= $2
		     GROUP  BY 1
		 ),
		 done_counts AS (
		     SELECT DATE_TRUNC('day', updated_at)::date AS day,
		            COUNT(*)::int                        AS cnt
		     FROM   tasks
		     WHERE  board_id IN (SELECT id FROM board_scope)
		       AND  ($3::text IS NULL OR board_id = $3)
		       AND  stage      = 'done'
		       AND  updated_at >= $2
		     GROUP  BY 1
		 )
		 SELECT
		     ds.day,
		     COALESCE(c.cnt, 0) AS created,
		     COALESCE(d.cnt, 0) AS completed
		 FROM   date_series   ds
		 LEFT   JOIN created_counts c ON c.day = ds.day
		 LEFT   JOIN done_counts    d ON d.day = ds.day
		 ORDER  BY ds.day ASC`,
		userID, since, boardID,
	)
	if err != nil {
		return nil, errs.Internal("failed to query task completion trend")
	}
	defer rows.Close()

	var points []model.TaskTrendPoint
	for rows.Next() {
		var day time.Time
		var p model.TaskTrendPoint
		if err := rows.Scan(&day, &p.Created, &p.Completed); err != nil {
			return nil, errs.Internal("failed to scan trend row")
		}
		p.Date = day.Format("2006-01-02")
		points = append(points, p)
	}
	if rows.Err() != nil {
		return nil, errs.Internal("task trend query error")
	}
	return points, nil
}

// ── Task distribution ─────────────────────────────────────────────────────────

// GetTaskDistribution returns current task counts grouped by stage and by
// priority. Two lightweight queries run sequentially; both share the same
// board scope logic.
func (r *AnalyticsRepo) GetTaskDistribution(
	ctx context.Context,
	userID string,
	boardID *string,
) (*model.TaskDistribution, error) {
	dist := &model.TaskDistribution{}

	// ── By stage ──────────────────────────────────────────────────────────────
	stageRows, err := r.db.Query(ctx,
		`SELECT stage, COUNT(*)::int AS count
		 FROM   tasks
		 WHERE  board_id IN (SELECT id FROM boards WHERE owner_id = $1)
		   AND  ($2::text IS NULL OR board_id = $2)
		 GROUP  BY stage
		 ORDER  BY stage`,
		userID, boardID,
	)
	if err != nil {
		return nil, errs.Internal("failed to query stage distribution")
	}
	defer stageRows.Close()

	for stageRows.Next() {
		var sc model.StageCount
		if err := stageRows.Scan(&sc.Stage, &sc.Count); err != nil {
			return nil, errs.Internal("failed to scan stage row")
		}
		dist.ByStage = append(dist.ByStage, sc)
	}
	if stageRows.Err() != nil {
		return nil, errs.Internal("stage distribution query error")
	}
	stageRows.Close()

	// ── By priority ───────────────────────────────────────────────────────────
	prioRows, err := r.db.Query(ctx,
		`SELECT priority, COUNT(*)::int AS count
		 FROM   tasks
		 WHERE  board_id IN (SELECT id FROM boards WHERE owner_id = $1)
		   AND  ($2::text IS NULL OR board_id = $2)
		 GROUP  BY priority
		 ORDER  BY priority`,
		userID, boardID,
	)
	if err != nil {
		return nil, errs.Internal("failed to query priority distribution")
	}
	defer prioRows.Close()

	for prioRows.Next() {
		var pc model.PriorityCount
		if err := prioRows.Scan(&pc.Priority, &pc.Count); err != nil {
			return nil, errs.Internal("failed to scan priority row")
		}
		dist.ByPriority = append(dist.ByPriority, pc)
	}
	if prioRows.Err() != nil {
		return nil, errs.Internal("priority distribution query error")
	}

	// Ensure slices are never nil in JSON output.
	if dist.ByStage == nil {
		dist.ByStage = []model.StageCount{}
	}
	if dist.ByPriority == nil {
		dist.ByPriority = []model.PriorityCount{}
	}
	return dist, nil
}

// ── Board activity ────────────────────────────────────────────────────────────

// GetBoardActivity returns one row per calendar day from since to today.
// tasks_created — new task rows inserted on that day.
// tasks_updated — existing tasks whose updated_at falls on that day but whose
//
//	created_at does not (i.e. real edits, not the initial insert).
func (r *AnalyticsRepo) GetBoardActivity(
	ctx context.Context,
	userID string,
	since time.Time,
	boardID *string,
) ([]model.BoardActivityPoint, error) {
	rows, err := r.db.Query(ctx,
		`WITH board_scope AS (
		     SELECT id FROM boards WHERE owner_id = $1
		 ),
		 date_series AS (
		     SELECT generate_series($2::timestamptz, NOW(), '1 day')::date AS day
		 ),
		 created AS (
		     SELECT DATE_TRUNC('day', created_at)::date AS day,
		            COUNT(*)::int                        AS cnt
		     FROM   tasks
		     WHERE  board_id IN (SELECT id FROM board_scope)
		       AND  ($3::text IS NULL OR board_id = $3)
		       AND  created_at >= $2
		     GROUP  BY 1
		 ),
		 updated AS (
		     SELECT DATE_TRUNC('day', updated_at)::date AS day,
		            COUNT(*)::int                        AS cnt
		     FROM   tasks
		     WHERE  board_id IN (SELECT id FROM board_scope)
		       AND  ($3::text IS NULL OR board_id = $3)
		       AND  updated_at >= $2
		       AND  DATE_TRUNC('day', updated_at) != DATE_TRUNC('day', created_at)
		     GROUP  BY 1
		 )
		 SELECT
		     ds.day,
		     COALESCE(c.cnt, 0) AS tasks_created,
		     COALESCE(u.cnt, 0) AS tasks_updated
		 FROM   date_series ds
		 LEFT   JOIN created c ON c.day = ds.day
		 LEFT   JOIN updated u ON u.day = ds.day
		 ORDER  BY ds.day ASC`,
		userID, since, boardID,
	)
	if err != nil {
		return nil, errs.Internal("failed to query board activity")
	}
	defer rows.Close()

	var points []model.BoardActivityPoint
	for rows.Next() {
		var day time.Time
		var p model.BoardActivityPoint
		if err := rows.Scan(&day, &p.TasksCreated, &p.TasksUpdated); err != nil {
			return nil, errs.Internal("failed to scan board activity row")
		}
		p.Date = day.Format("2006-01-02")
		points = append(points, p)
	}
	if rows.Err() != nil {
		return nil, errs.Internal("board activity query error")
	}
	return points, nil
}

// ── Team contribution ─────────────────────────────────────────────────────────

// GetTeamContribution returns one row per unique task creator within the
// scoped board(s). Counts cover: total tasks created, tasks in 'done',
// and tasks past their due_date that are not yet done (overdue).
func (r *AnalyticsRepo) GetTeamContribution(
	ctx context.Context,
	userID string,
	boardID *string,
) ([]model.TeamMemberContribution, error) {
	rows, err := r.db.Query(ctx,
		`SELECT
		     t.created_by                                                    AS user_id,
		     u.name                                                          AS user_name,
		     COUNT(*)::int                                                   AS tasks_created,
		     COUNT(*) FILTER (WHERE t.stage = 'done')::int                  AS tasks_completed,
		     COUNT(*) FILTER (
		         WHERE t.due_date < NOW() AND t.stage != 'done'
		     )::int                                                          AS tasks_overdue
		 FROM   tasks t
		 JOIN   users u ON u.id = t.created_by
		 WHERE  t.board_id IN (SELECT id FROM boards WHERE owner_id = $1)
		   AND  ($2::text IS NULL OR t.board_id = $2)
		 GROUP  BY t.created_by, u.name
		 ORDER  BY tasks_created DESC`,
		userID, boardID,
	)
	if err != nil {
		return nil, errs.Internal("failed to query team contribution")
	}
	defer rows.Close()

	var contribs []model.TeamMemberContribution
	for rows.Next() {
		var c model.TeamMemberContribution
		if err := rows.Scan(
			&c.UserID, &c.UserName,
			&c.Created, &c.Completed, &c.Overdue,
		); err != nil {
			return nil, errs.Internal("failed to scan contribution row")
		}
		contribs = append(contribs, c)
	}
	if rows.Err() != nil {
		return nil, errs.Internal("team contribution query error")
	}
	if contribs == nil {
		contribs = []model.TeamMemberContribution{}
	}
	return contribs, nil
}
