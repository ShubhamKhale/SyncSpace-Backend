package database

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"syncspace-backend/errs"
	"syncspace-backend/model"
)

// TaskFilter holds optional query filters for listing tasks.
// A zero-value field means "no filter on this column".
type TaskFilter struct {
	Stage      string // "" = all stages
	Priority   string // "" = all priorities
	AssigneeID string // "" = all assignees
}

// TaskRepo handles all database operations for the Task entity.
type TaskRepo struct {
	db *pgxpool.Pool
}

// NewTaskRepo creates a TaskRepo with the provided connection pool.
func NewTaskRepo(db *pgxpool.Pool) *TaskRepo {
	return &TaskRepo{db: db}
}

// scanTask reads one task row in the standard column order.
func scanTask(row pgx.Row) (*model.Task, error) {
	t := &model.Task{}
	err := row.Scan(
		&t.ID, &t.BoardID, &t.Title, &t.Description,
		&t.Stage, &t.Priority, &t.AssigneeID,
		&t.CreatedBy, &t.Position, &t.DueDate,
		&t.CreatedAt, &t.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errs.NotFound("task not found")
		}
		return nil, errs.Internal("failed to scan task")
	}
	return t, nil
}

// InsertTask persists a new task. Position is set to the next available slot
// in the given stage column so new tasks always appear at the bottom.
func (r *TaskRepo) InsertTask(ctx context.Context, task *model.Task) error {
	// Append-to-bottom: count existing tasks in this stage to derive position.
	var maxPos int
	_ = r.db.QueryRow(ctx,
		`SELECT COALESCE(MAX(position) + 1, 0)
		 FROM tasks WHERE board_id = $1 AND stage = $2`,
		task.BoardID, task.Stage,
	).Scan(&maxPos)
	task.Position = maxPos

	_, err := r.db.Exec(ctx,
		`INSERT INTO tasks
		     (id, board_id, title, description, stage, priority,
		      assignee_id, created_by, position, due_date, created_at, updated_at)
		 VALUES ($1,$2,$3,$4,$5,$6,NULLIF($7,''),$8,$9,$10,$11,$12)`,
		task.ID, task.BoardID, task.Title, task.Description,
		task.Stage, task.Priority, task.AssigneeID, task.CreatedBy,
		task.Position, task.DueDate, task.CreatedAt, task.UpdatedAt,
	)
	if err != nil {
		return errs.Internal("failed to insert task")
	}
	return nil
}

// GetTaskByID returns a single task, or errs.NotFound.
func (r *TaskRepo) GetTaskByID(ctx context.Context, id string) (*model.Task, error) {
	row := r.db.QueryRow(ctx,
		`SELECT id, board_id, title, description, stage, priority,
		        COALESCE(assignee_id,''), created_by, position, due_date, created_at, updated_at
		 FROM tasks WHERE id = $1`,
		id,
	)
	return scanTask(row)
}

// GetTasksByBoard returns all tasks for a board with optional column filtering.
// Results are ordered by stage then position so the client gets Kanban-ready data.
func (r *TaskRepo) GetTasksByBoard(ctx context.Context, boardID string, f TaskFilter) ([]model.Task, error) {
	// Use SQL NULLs for omitted filters so one query handles all combinations.
	var stageArg, priorityArg, assigneeArg *string
	if f.Stage != "" {
		stageArg = &f.Stage
	}
	if f.Priority != "" {
		priorityArg = &f.Priority
	}
	if f.AssigneeID != "" {
		assigneeArg = &f.AssigneeID
	}

	rows, err := r.db.Query(ctx,
		`SELECT id, board_id, title, description, stage, priority,
		        COALESCE(assignee_id,''), created_by, position, due_date, created_at, updated_at
		 FROM   tasks
		 WHERE  board_id  = $1
		   AND ($2::text IS NULL OR stage       = $2)
		   AND ($3::text IS NULL OR priority    = $3)
		   AND ($4::text IS NULL OR assignee_id = $4)
		 ORDER BY stage, position ASC`,
		boardID, stageArg, priorityArg, assigneeArg,
	)
	if err != nil {
		return nil, errs.Internal("failed to query tasks")
	}
	defer rows.Close()

	var tasks []model.Task
	for rows.Next() {
		var t model.Task
		if err := rows.Scan(
			&t.ID, &t.BoardID, &t.Title, &t.Description,
			&t.Stage, &t.Priority, &t.AssigneeID,
			&t.CreatedBy, &t.Position, &t.DueDate,
			&t.CreatedAt, &t.UpdatedAt,
		); err != nil {
			return nil, errs.Internal("failed to scan task row")
		}
		tasks = append(tasks, t)
	}
	if rows.Err() != nil {
		return nil, errs.Internal("task query error")
	}
	return tasks, nil
}

// UpdateTask persists changes to mutable task fields (title, description,
// priority, assignee_id, due_date).
func (r *TaskRepo) UpdateTask(ctx context.Context, task *model.Task) error {
	tag, err := r.db.Exec(ctx,
		`UPDATE tasks
		 SET    title=$2, description=$3, priority=$4,
		        assignee_id=NULLIF($5,''), due_date=$6, updated_at=$7
		 WHERE  id=$1`,
		task.ID, task.Title, task.Description, task.Priority,
		task.AssigneeID, task.DueDate, task.UpdatedAt,
	)
	if err != nil {
		return errs.Internal("failed to update task")
	}
	if tag.RowsAffected() == 0 {
		return errs.NotFound("task not found")
	}
	return nil
}

// GetUpcomingTasksByUser returns non-done tasks assigned to or created by the
// given user that are due within the next 7 days, ordered by due_date ascending.
func (r *TaskRepo) GetUpcomingTasksByUser(ctx context.Context, userID string, limit int) ([]model.Task, error) {
	rows, err := r.db.Query(ctx,
		`SELECT id, board_id, title, description, stage, priority,
		        COALESCE(assignee_id,''), created_by, position, due_date, created_at, updated_at
		 FROM   tasks
		 WHERE  (assignee_id = $1 OR created_by = $1)
		   AND  stage    != 'done'
		   AND  due_date IS NOT NULL
		   AND  due_date BETWEEN NOW() AND NOW() + INTERVAL '7 days'
		 ORDER  BY due_date ASC
		 LIMIT  $2`,
		userID, limit,
	)
	if err != nil {
		return nil, errs.Internal("failed to query upcoming tasks")
	}
	defer rows.Close()

	var tasks []model.Task
	for rows.Next() {
		var t model.Task
		if err := rows.Scan(
			&t.ID, &t.BoardID, &t.Title, &t.Description,
			&t.Stage, &t.Priority, &t.AssigneeID,
			&t.CreatedBy, &t.Position, &t.DueDate,
			&t.CreatedAt, &t.UpdatedAt,
		); err != nil {
			return nil, errs.Internal("failed to scan upcoming task row")
		}
		tasks = append(tasks, t)
	}
	if rows.Err() != nil {
		return nil, errs.Internal("upcoming task query error")
	}
	return tasks, nil
}

// UpdateTaskStage moves a task to a new stage at the specified position.
// Runs inside a transaction to keep all position values contiguous:
//
//  1. Close the gap in the source stage by shifting later tasks up.
//  2. Open a slot in the destination stage by shifting tasks from newPosition down.
//  3. Place the task at the new stage and position.
func (r *TaskRepo) UpdateTaskStage(ctx context.Context, taskID, boardID, newStage string, newPosition int) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return errs.Internal("failed to begin transaction")
	}
	defer tx.Rollback(ctx) //nolint:errcheck — rollback on all non-commit paths

	// ── 1. Fetch current state ────────────────────────────────────────────────
	var currentStage string
	var currentPos int
	err = tx.QueryRow(ctx,
		`SELECT stage, position FROM tasks WHERE id = $1 AND board_id = $2`,
		taskID, boardID,
	).Scan(&currentStage, &currentPos)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return errs.NotFound("task not found")
		}
		return errs.Internal("failed to query task")
	}

	// ── 2. Close gap in old stage ─────────────────────────────────────────────
	if _, err = tx.Exec(ctx,
		`UPDATE tasks
		 SET    position = position - 1
		 WHERE  board_id = $1 AND stage = $2 AND position > $3 AND id != $4`,
		boardID, currentStage, currentPos, taskID,
	); err != nil {
		return errs.Internal("failed to reorder source column")
	}

	// ── 3. Open slot in destination stage ─────────────────────────────────────
	if _, err = tx.Exec(ctx,
		`UPDATE tasks
		 SET    position = position + 1
		 WHERE  board_id = $1 AND stage = $2 AND position >= $3 AND id != $4`,
		boardID, newStage, newPosition, taskID,
	); err != nil {
		return errs.Internal("failed to reorder destination column")
	}

	// ── 4. Place task ─────────────────────────────────────────────────────────
	now := time.Now()
	tag, err := tx.Exec(ctx,
		`UPDATE tasks SET stage=$2, position=$3, updated_at=$4 WHERE id=$1`,
		taskID, newStage, newPosition, now,
	)
	if err != nil {
		return errs.Internal("failed to update task stage")
	}
	if tag.RowsAffected() == 0 {
		return errs.NotFound("task not found")
	}

	return tx.Commit(ctx)
}
