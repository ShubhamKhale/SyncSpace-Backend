package database

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"syncspace-backend/errs"
	"syncspace-backend/model"
)

// AnalyticsRepo provides read-only aggregation queries for the analytics
// endpoints. All queries are scoped to boards in the caller's org;
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
	orgID string,
	since time.Time,
	boardID *string,
) ([]model.TaskTrendPoint, error) {
	rows, err := r.db.Query(ctx,
		`WITH board_scope AS (
		     SELECT id FROM public.boards WHERE org_id = $1
		 ),
		 date_series AS (
		     SELECT generate_series($2::timestamptz, NOW(), '1 day')::date AS day
		 ),
		 created_counts AS (
		     SELECT DATE_TRUNC('day', created_at)::date AS day,
		            COUNT(*)::int                        AS cnt
		     FROM   public.tasks
		     WHERE  board_id IN (SELECT id FROM board_scope)
		       AND  ($3::text IS NULL OR board_id = $3)
		       AND  created_at >= $2
		     GROUP  BY 1
		 ),
		 done_counts AS (
		     SELECT DATE_TRUNC('day', updated_at)::date AS day,
		            COUNT(*)::int                        AS cnt
		     FROM   public.tasks
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
		orgID, since, boardID,
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
	orgID string,
	boardID *string,
) (*model.TaskDistribution, error) {
	dist := &model.TaskDistribution{}

	// ── By stage ──────────────────────────────────────────────────────────────
	stageRows, err := r.db.Query(ctx,
		`SELECT stage, COUNT(*)::int AS count
		 FROM   public.tasks
		 WHERE  board_id IN (SELECT id FROM public.boards WHERE org_id = $1)
		   AND  ($2::text IS NULL OR board_id = $2)
		 GROUP  BY stage
		 ORDER  BY stage`,
		orgID, boardID,
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
		 FROM   public.tasks
		 WHERE  board_id IN (SELECT id FROM public.boards WHERE org_id = $1)
		   AND  ($2::text IS NULL OR board_id = $2)
		 GROUP  BY priority
		 ORDER  BY priority`,
		orgID, boardID,
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
	orgID string,
	since time.Time,
	boardID *string,
) ([]model.BoardActivityPoint, error) {
	rows, err := r.db.Query(ctx,
		`WITH board_scope AS (
		     SELECT id FROM public.boards WHERE org_id = $1
		 ),
		 date_series AS (
		     SELECT generate_series($2::timestamptz, NOW(), '1 day')::date AS day
		 ),
		 created AS (
		     SELECT DATE_TRUNC('day', created_at)::date AS day,
		            COUNT(*)::int                        AS cnt
		     FROM   public.tasks
		     WHERE  board_id IN (SELECT id FROM board_scope)
		       AND  ($3::text IS NULL OR board_id = $3)
		       AND  created_at >= $2
		     GROUP  BY 1
		 ),
		 updated AS (
		     SELECT DATE_TRUNC('day', updated_at)::date AS day,
		            COUNT(*)::int                        AS cnt
		     FROM   public.tasks
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
		orgID, since, boardID,
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
	orgID string,
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
		 FROM   public.tasks t
		 JOIN   public.users u ON u.id = t.created_by
		 WHERE  t.board_id IN (SELECT id FROM public.boards WHERE org_id = $1)
		   AND  ($2::text IS NULL OR t.board_id = $2)
		 GROUP  BY t.created_by, u.name
		 ORDER  BY tasks_created DESC`,
		orgID, boardID,
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

// ── New dashboard analytics queries ──────────────────────────────────────────

// GetMonthlyTaskTrend returns task creation and completion counts for the last
// 7 months (current month + 6 prior), oldest first.
// Month is formatted as a 3-letter abbreviation ("Jan", "Feb", etc.).
func (r *AnalyticsRepo) GetMonthlyTaskTrend(ctx context.Context, orgID string) ([]model.MonthlyTrendPoint, error) {
	rows, err := r.db.Query(ctx,
		`WITH months AS (
		     SELECT generate_series(
		         date_trunc('month', NOW()) - INTERVAL '6 months',
		         date_trunc('month', NOW()),
		         '1 month'
		     ) AS month_start
		 ),
		 org_boards AS (
		     SELECT id FROM public.boards WHERE org_id = $1
		 ),
		 created_counts AS (
		     SELECT date_trunc('month', created_at) AS m, COUNT(*)::int AS cnt
		     FROM   public.tasks
		     WHERE  board_id IN (SELECT id FROM org_boards)
		       AND  created_at >= date_trunc('month', NOW()) - INTERVAL '6 months'
		     GROUP  BY 1
		 ),
		 done_counts AS (
		     SELECT date_trunc('month', updated_at) AS m, COUNT(*)::int AS cnt
		     FROM   public.tasks
		     WHERE  board_id IN (SELECT id FROM org_boards)
		       AND  stage = 'done'
		       AND  updated_at >= date_trunc('month', NOW()) - INTERVAL '6 months'
		     GROUP  BY 1
		 )
		 SELECT
		     TO_CHAR(months.month_start, 'Mon') AS month,
		     COALESCE(c.cnt, 0)                 AS created,
		     COALESCE(d.cnt, 0)                 AS completed
		 FROM   months
		 LEFT JOIN created_counts c ON c.m = months.month_start
		 LEFT JOIN done_counts    d ON d.m = months.month_start
		 ORDER  BY months.month_start ASC`,
		orgID,
	)
	if err != nil {
		return nil, errs.Internal("failed to query monthly task trend")
	}
	defer rows.Close()

	var points []model.MonthlyTrendPoint
	for rows.Next() {
		var p model.MonthlyTrendPoint
		if err := rows.Scan(&p.Month, &p.Created, &p.Completed); err != nil {
			return nil, errs.Internal("failed to scan monthly trend row")
		}
		points = append(points, p)
	}
	if rows.Err() != nil {
		return nil, errs.Internal("monthly task trend query error")
	}
	return points, nil
}

// GetTaskDistributionByStage returns task counts grouped by stage with human-readable labels.
func (r *AnalyticsRepo) GetTaskDistributionByStage(ctx context.Context, orgID string) ([]model.DistributionItem, error) {
	rows, err := r.db.Query(ctx,
		`SELECT
		     CASE stage
		         WHEN 'todo'        THEN 'Todo'
		         WHEN 'in_progress' THEN 'In Progress'
		         WHEN 'done'        THEN 'Done'
		         ELSE stage
		     END  AS name,
		     COUNT(*)::int AS value
		 FROM   public.tasks
		 WHERE  board_id IN (SELECT id FROM public.boards WHERE org_id = $1)
		 GROUP  BY stage
		 ORDER  BY value DESC`,
		orgID,
	)
	if err != nil {
		return nil, errs.Internal("failed to query task distribution")
	}
	defer rows.Close()

	var items []model.DistributionItem
	for rows.Next() {
		var item model.DistributionItem
		if err := rows.Scan(&item.Name, &item.Value); err != nil {
			return nil, errs.Internal("failed to scan distribution row")
		}
		items = append(items, item)
	}
	if rows.Err() != nil {
		return nil, errs.Internal("task distribution query error")
	}
	if items == nil {
		items = []model.DistributionItem{}
	}
	return items, nil
}

// GetBoardActivityStats returns the top 5 most active boards in the org by
// activity log count in the last 30 days.
func (r *AnalyticsRepo) GetBoardActivityStats(ctx context.Context, orgID string) ([]model.BoardActivityItem, error) {
	rows, err := r.db.Query(ctx,
		`WITH org_boards AS (
		     SELECT id, SUBSTRING(title, 1, 20) AS name
		     FROM   public.boards WHERE org_id = $1
		 ),
		 board_edits AS (
		     SELECT al.entity_id AS board_id, COUNT(*)::int AS cnt
		     FROM   public.activity_logs al
		     WHERE  al.entity_id IN (SELECT id FROM org_boards)
		       AND  al.entity_type = 'board'
		       AND  al.created_at >= NOW() - INTERVAL '30 days'
		     GROUP  BY al.entity_id
		 ),
		 task_edits AS (
		     SELECT b.id AS board_id, COUNT(*)::int AS cnt
		     FROM   public.activity_logs al
		     JOIN   public.tasks t  ON t.id  = al.entity_id
		     JOIN   public.boards b ON b.id  = t.board_id
		     WHERE  b.id IN (SELECT id FROM org_boards)
		       AND  al.entity_type = 'task'
		       AND  al.created_at >= NOW() - INTERVAL '30 days'
		     GROUP  BY b.id
		 )
		 SELECT
		     ob.name,
		     COALESCE(be.cnt, 0) + COALESCE(te.cnt, 0) AS edits,
		     0                                           AS comments,
		     0                                           AS shares
		 FROM   org_boards  ob
		 LEFT JOIN board_edits be ON be.board_id = ob.id
		 LEFT JOIN task_edits  te ON te.board_id = ob.id
		 ORDER  BY edits DESC
		 LIMIT  5`,
		orgID,
	)
	if err != nil {
		return nil, errs.Internal("failed to query board activity stats")
	}
	defer rows.Close()

	var items []model.BoardActivityItem
	for rows.Next() {
		var item model.BoardActivityItem
		if err := rows.Scan(&item.Name, &item.Edits, &item.Comments, &item.Shares); err != nil {
			return nil, errs.Internal("failed to scan board activity row")
		}
		items = append(items, item)
	}
	if rows.Err() != nil {
		return nil, errs.Internal("board activity stats query error")
	}
	if items == nil {
		items = []model.BoardActivityItem{}
	}
	return items, nil
}

// GetTeamContributionByPhase returns a radar-chart dataset with one row per
// phase (Planning/Development/Deployment mapped from task stages), and dynamic
// columns for the top 3 members by task count.
// Single query — eliminates the previous N+1 (1 + 3 round-trips) pattern.
func (r *AnalyticsRepo) GetTeamContributionByPhase(ctx context.Context, orgID string) (*model.TeamContributionResult, error) {
	rows, err := r.db.Query(ctx,
		`WITH org_boards AS (
		     SELECT id FROM public.boards WHERE org_id = $1
		 ),
		 top_members AS (
		     SELECT u.id, u.name, COUNT(*) AS total
		     FROM   public.tasks t
		     JOIN   public.users u ON u.id = COALESCE(t.assignee_id, t.created_by)
		     WHERE  t.board_id IN (SELECT id FROM org_boards)
		     GROUP  BY u.id, u.name
		     ORDER  BY total DESC
		     LIMIT  3
		 )
		 SELECT
		     u.id,
		     u.name,
		     COUNT(*) FILTER (WHERE t.stage = 'todo')::int        AS planning,
		     COUNT(*) FILTER (WHERE t.stage = 'in_progress')::int AS development,
		     COUNT(*) FILTER (WHERE t.stage = 'done')::int        AS deployment
		 FROM   public.tasks t
		 JOIN   public.users u ON u.id = COALESCE(t.assignee_id, t.created_by)
		 WHERE  t.board_id IN (SELECT id FROM org_boards)
		   AND  u.id IN (SELECT id FROM top_members)
		 GROUP  BY u.id, u.name
		 ORDER  BY COUNT(*) DESC`,
		orgID,
	)
	if err != nil {
		return nil, errs.Internal("failed to query team contribution by phase")
	}
	defer rows.Close()

	type memberRow struct {
		Name        string
		Planning    int
		Development int
		Deployment  int
	}

	var memberRows []memberRow
	for rows.Next() {
		var id string
		var mr memberRow
		if err := rows.Scan(&id, &mr.Name, &mr.Planning, &mr.Development, &mr.Deployment); err != nil {
			return nil, errs.Internal("failed to scan team contribution row")
		}
		memberRows = append(memberRows, mr)
	}
	if rows.Err() != nil {
		return nil, errs.Internal("team contribution query error")
	}

	result := &model.TeamContributionResult{
		Data:    []map[string]any{},
		Members: []string{},
	}
	if len(memberRows) == 0 {
		return result, nil
	}

	for _, mr := range memberRows {
		result.Members = append(result.Members, mr.Name)
	}

	phases := []struct {
		Label string
		Field func(memberRow) int
	}{
		{"Planning", func(mr memberRow) int { return mr.Planning }},
		{"Development", func(mr memberRow) int { return mr.Development }},
		{"Deployment", func(mr memberRow) int { return mr.Deployment }},
	}

	for _, phase := range phases {
		row := map[string]any{"phase": phase.Label}
		for _, mr := range memberRows {
			row[mr.Name] = phase.Field(mr)
		}
		result.Data = append(result.Data, row)
	}

	return result, nil
}
