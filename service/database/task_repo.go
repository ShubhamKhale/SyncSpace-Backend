package database

import (
	"context"
	"encoding/json"
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

// taskCols is the canonical SELECT column list for task rows.
const taskCols = `id, board_id, title, description, stage, priority,
       COALESCE(assignee_id,''), created_by, position, due_date, start_date,
       tags, time_estimate, reference_link, flow_diagram_link,
       subtasks, attachments, created_at, updated_at`

// scanTaskRow reads one task row in taskCols order.
func scanTaskRow(scan func(dest ...any) error) (*model.Task, error) {
	t := &model.Task{}
	var tagsRaw, subtasksRaw, attachmentsRaw json.RawMessage
	err := scan(
		&t.ID, &t.BoardID, &t.Title, &t.Description,
		&t.Stage, &t.Priority, &t.AssigneeID,
		&t.CreatedBy, &t.Position, &t.DueDate, &t.StartDate,
		&tagsRaw, &t.TimeEstimate, &t.ReferenceLink, &t.FlowDiagramLink,
		&subtasksRaw, &attachmentsRaw,
		&t.CreatedAt, &t.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(tagsRaw, &t.Tags); err != nil || t.Tags == nil {
		t.Tags = []string{}
	}
	if err := json.Unmarshal(subtasksRaw, &t.Subtasks); err != nil || t.Subtasks == nil {
		t.Subtasks = []model.Subtask{}
	}
	if err := json.Unmarshal(attachmentsRaw, &t.Attachments); err != nil || t.Attachments == nil {
		t.Attachments = []model.Attachment{}
	}
	return t, nil
}

// InsertTask persists a new task. Position is set to the next available slot
// in the given stage column so new tasks always appear at the bottom.
func (r *TaskRepo) InsertTask(ctx context.Context, task *model.Task) error {
	task.Position = r.NextPosition(ctx, task.BoardID, task.Stage)

	tagsJSON, _ := json.Marshal(task.Tags)
	if task.Tags == nil {
		tagsJSON = []byte("[]")
	}
	subtasksJSON, _ := json.Marshal(task.Subtasks)
	if task.Subtasks == nil {
		subtasksJSON = []byte("[]")
	}
	attachmentsJSON, _ := json.Marshal(task.Attachments)
	if task.Attachments == nil {
		attachmentsJSON = []byte("[]")
	}

	_, err := r.db.Exec(ctx,
		`INSERT INTO public.tasks
		     (id, board_id, title, description, stage, priority,
		      assignee_id, created_by, position, due_date, start_date,
		      tags, time_estimate, reference_link, flow_diagram_link,
		      subtasks, attachments, created_at, updated_at)
		 VALUES ($1,$2,$3,$4,$5,$6,NULLIF($7,''),$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19)`,
		task.ID, task.BoardID, task.Title, task.Description,
		task.Stage, task.Priority, task.AssigneeID, task.CreatedBy,
		task.Position, task.DueDate, task.StartDate,
		json.RawMessage(tagsJSON), task.TimeEstimate, task.ReferenceLink, task.FlowDiagramLink,
		json.RawMessage(subtasksJSON), json.RawMessage(attachmentsJSON),
		task.CreatedAt, task.UpdatedAt,
	)
	if err != nil {
		return errs.Internal("failed to insert task")
	}
	return nil
}

// GetTaskByID returns a single task, or errs.NotFound.
func (r *TaskRepo) GetTaskByID(ctx context.Context, id string) (*model.Task, error) {
	row := r.db.QueryRow(ctx,
		`SELECT `+taskCols+` FROM public.tasks WHERE id = $1`, id,
	)
	t, err := scanTaskRow(row.Scan)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errs.NotFound("task not found")
		}
		return nil, errs.Internal("failed to scan task")
	}
	return t, nil
}

// GetTasksByBoard returns all tasks for a board with optional column filtering.
// Results are ordered by stage then position so the client gets Kanban-ready data.
func (r *TaskRepo) GetTasksByBoard(ctx context.Context, boardID string, f TaskFilter) ([]model.Task, error) {
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
		`SELECT `+taskCols+`
		 FROM   public.tasks
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
		t, err := scanTaskRow(rows.Scan)
		if err != nil {
			return nil, errs.Internal("failed to scan task row")
		}
		tasks = append(tasks, *t)
	}
	if rows.Err() != nil {
		return nil, errs.Internal("task query error")
	}
	return tasks, nil
}

// NextPosition returns the next free position (bottom slot) in a board's stage column.
func (r *TaskRepo) NextPosition(ctx context.Context, boardID, stage string) int {
	var pos int
	_ = r.db.QueryRow(ctx,
		`SELECT COALESCE(MAX(position) + 1, 0)
		 FROM public.tasks WHERE board_id = $1 AND stage = $2`,
		boardID, stage,
	).Scan(&pos)
	return pos
}

// UpdateTask persists changes to mutable task fields. Stage and position are
// not written here — stage changes go through UpdateTaskStage to keep column
// positions contiguous.
func (r *TaskRepo) UpdateTask(ctx context.Context, task *model.Task) error {
	tagsJSON, _ := json.Marshal(task.Tags)
	if task.Tags == nil {
		tagsJSON = []byte("[]")
	}
	subtasksJSON, _ := json.Marshal(task.Subtasks)
	if task.Subtasks == nil {
		subtasksJSON = []byte("[]")
	}

	tag, err := r.db.Exec(ctx,
		`UPDATE public.tasks
		 SET    title=$2, description=$3, priority=$4,
		        assignee_id=NULLIF($5,''), due_date=$6, start_date=$7,
		        tags=$8, time_estimate=$9, reference_link=$10, flow_diagram_link=$11,
		        subtasks=$12, updated_at=$13
		 WHERE  id=$1`,
		task.ID, task.Title, task.Description, task.Priority,
		task.AssigneeID, task.DueDate, task.StartDate,
		json.RawMessage(tagsJSON), task.TimeEstimate, task.ReferenceLink, task.FlowDiagramLink,
		json.RawMessage(subtasksJSON), task.UpdatedAt,
	)
	if err != nil {
		return errs.Internal("failed to update task")
	}
	if tag.RowsAffected() == 0 {
		return errs.NotFound("task not found")
	}
	return nil
}

// GetTaskOrgID returns the org that owns the task's board, or errs.NotFound.
func (r *TaskRepo) GetTaskOrgID(ctx context.Context, taskID string) (string, error) {
	var orgID string
	err := r.db.QueryRow(ctx,
		`SELECT b.org_id
		 FROM   public.tasks t
		 JOIN   public.boards b ON b.id = t.board_id
		 WHERE  t.id = $1`,
		taskID,
	).Scan(&orgID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", errs.NotFound("task not found")
		}
		return "", errs.Internal("failed to resolve task org")
	}
	return orgID, nil
}

// BoardInOrg reports whether the board exists and belongs to the given org.
func (r *TaskRepo) BoardInOrg(ctx context.Context, boardID, orgID string) (bool, error) {
	var ok bool
	err := r.db.QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM public.boards WHERE id = $1 AND org_id = $2)`,
		boardID, orgID,
	).Scan(&ok)
	if err != nil {
		return false, errs.Internal("failed to verify board org")
	}
	return ok, nil
}

// IsActiveOrgMember reports whether userID is an active member of orgID.
func (r *TaskRepo) IsActiveOrgMember(ctx context.Context, orgID, userID string) (bool, error) {
	var ok bool
	err := r.db.QueryRow(ctx,
		`SELECT EXISTS (
		     SELECT 1 FROM public.organization_members
		     WHERE organization_id = $1 AND user_id = $2 AND status = 'active')`,
		orgID, userID,
	).Scan(&ok)
	if err != nil {
		return false, errs.Internal("failed to verify org membership")
	}
	return ok, nil
}

// DeleteTask removes a task and closes the gap it leaves in its stage column.
func (r *TaskRepo) DeleteTask(ctx context.Context, taskID string) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return errs.Internal("failed to begin transaction")
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	var boardID, stage string
	var pos int
	err = tx.QueryRow(ctx,
		`DELETE FROM public.tasks WHERE id = $1 RETURNING board_id, stage, position`,
		taskID,
	).Scan(&boardID, &stage, &pos)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return errs.NotFound("task not found")
		}
		return errs.Internal("failed to delete task")
	}

	if _, err = tx.Exec(ctx,
		`UPDATE public.tasks
		 SET    position = position - 1
		 WHERE  board_id = $1 AND stage = $2 AND position > $3`,
		boardID, stage, pos,
	); err != nil {
		return errs.Internal("failed to reorder column after delete")
	}

	return tx.Commit(ctx)
}

// GetUpcomingTasksByUser returns non-done tasks assigned to or created by the
// given user that are due within the next 7 days, ordered by due_date ascending.
func (r *TaskRepo) GetUpcomingTasksByUser(ctx context.Context, userID string, limit int) ([]model.Task, error) {
	rows, err := r.db.Query(ctx,
		`SELECT `+taskCols+`
		 FROM (
		     SELECT id, board_id, title, description, stage, priority,
		            assignee_id, created_by, position, due_date, start_date,
		            tags, time_estimate, reference_link, flow_diagram_link,
		            subtasks, attachments, created_at, updated_at
		     FROM   public.tasks
		     WHERE  assignee_id = $1
		       AND  stage NOT IN ('Deployment')
		       AND  due_date BETWEEN NOW() AND NOW() + INTERVAL '7 days'
		     UNION ALL
		     SELECT id, board_id, title, description, stage, priority,
		            assignee_id, created_by, position, due_date, start_date,
		            tags, time_estimate, reference_link, flow_diagram_link,
		            subtasks, attachments, created_at, updated_at
		     FROM   public.tasks
		     WHERE  created_by = $1
		       AND  assignee_id IS NULL
		       AND  stage NOT IN ('Deployment')
		       AND  due_date BETWEEN NOW() AND NOW() + INTERVAL '7 days'
		 ) sub
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
		t, err := scanTaskRow(rows.Scan)
		if err != nil {
			return nil, errs.Internal("failed to scan upcoming task row")
		}
		tasks = append(tasks, *t)
	}
	if rows.Err() != nil {
		return nil, errs.Internal("upcoming task query error")
	}
	return tasks, nil
}

// UpdateTaskStage moves a task to a new stage at the specified position.
// Runs inside a transaction to keep all position values contiguous.
func (r *TaskRepo) UpdateTaskStage(ctx context.Context, taskID, boardID, newStage string, newPosition int) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return errs.Internal("failed to begin transaction")
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	var currentStage string
	var currentPos int
	err = tx.QueryRow(ctx,
		`SELECT stage, position FROM public.tasks WHERE id = $1 AND board_id = $2`,
		taskID, boardID,
	).Scan(&currentStage, &currentPos)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return errs.NotFound("task not found")
		}
		return errs.Internal("failed to query task")
	}

	if _, err = tx.Exec(ctx,
		`UPDATE public.tasks
		 SET    position = position - 1
		 WHERE  board_id = $1 AND stage = $2 AND position > $3 AND id != $4`,
		boardID, currentStage, currentPos, taskID,
	); err != nil {
		return errs.Internal("failed to reorder source column")
	}

	if _, err = tx.Exec(ctx,
		`UPDATE public.tasks
		 SET    position = position + 1
		 WHERE  board_id = $1 AND stage = $2 AND position >= $3 AND id != $4`,
		boardID, newStage, newPosition, taskID,
	); err != nil {
		return errs.Internal("failed to reorder destination column")
	}

	now := time.Now()
	tag, err := tx.Exec(ctx,
		`UPDATE public.tasks SET stage=$2, position=$3, updated_at=$4 WHERE id=$1`,
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
