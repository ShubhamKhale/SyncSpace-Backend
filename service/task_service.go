package service

import (
	"context"
	"time"

	"syncspace-backend/errs"
	"syncspace-backend/model"
	"syncspace-backend/pkg/utils"
	"syncspace-backend/service/database"
)

// valid* maps are used for input validation in the service layer.
var validStages = map[string]bool{
	"todo":        true,
	"in_progress": true,
	"done":        true,
}

var validPriorities = map[string]bool{
	"low":    true,
	"medium": true,
	"high":   true,
}

// TaskService handles task business logic.
type TaskService struct {
	repo   *database.TaskRepo
	logger *ActivityLogger
}

// NewTaskService creates a TaskService backed by the given TaskRepo.
func NewTaskService(repo *database.TaskRepo, logger *ActivityLogger) *TaskService {
	return &TaskService{repo: repo, logger: logger}
}

// CreateTask validates input and persists a new task on the given board.
// Stage defaults to "todo", priority defaults to "medium" if omitted.
func (s *TaskService) CreateTask(ctx context.Context, createdBy, boardID, title, description, stage, priority, assigneeID string, dueDate *time.Time) (*model.Task, error) {
	if title == "" {
		return nil, errs.BadRequest("title is required")
	}
	if stage == "" {
		stage = "todo"
	}
	if priority == "" {
		priority = "medium"
	}
	if !validStages[stage] {
		return nil, errs.BadRequest("stage must be one of: todo, in_progress, done")
	}
	if !validPriorities[priority] {
		return nil, errs.BadRequest("priority must be one of: low, medium, high")
	}

	now := time.Now()
	task := &model.Task{
		ID:          utils.NewUUID(),
		BoardID:     boardID,
		Title:       title,
		Description: description,
		Stage:       stage,
		Priority:    priority,
		AssigneeID:  assigneeID,
		CreatedBy:   createdBy,
		DueDate:     dueDate,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	if err := s.repo.InsertTask(ctx, task); err != nil {
		return nil, err
	}

	meta := map[string]any{"board_id": boardID, "title": title, "stage": stage}
	if assigneeID != "" {
		meta["assignee_id"] = assigneeID
	}
	s.logger.LogTask(ctx, createdBy, task.ID, ActionCreated, meta)

	return task, nil
}

// GetTasks returns tasks for a board with optional filtering.
// Any empty filter field is ignored (matches all values).
func (s *TaskService) GetTasks(ctx context.Context, boardID string, filter database.TaskFilter) ([]model.Task, error) {
	if filter.Stage != "" && !validStages[filter.Stage] {
		return nil, errs.BadRequest("stage must be one of: todo, in_progress, done")
	}
	if filter.Priority != "" && !validPriorities[filter.Priority] {
		return nil, errs.BadRequest("priority must be one of: low, medium, high")
	}

	tasks, err := s.repo.GetTasksByBoard(ctx, boardID, filter)
	if err != nil {
		return nil, err
	}
	if tasks == nil {
		tasks = []model.Task{}
	}
	return tasks, nil
}

// UpdateTask applies partial updates to mutable fields.
// Nil pointer fields are left unchanged.
func (s *TaskService) UpdateTask(
	ctx context.Context,
	taskID string,
	title, description, priority, assigneeID *string,
	dueDate **time.Time,
) (*model.Task, error) {
	task, err := s.repo.GetTaskByID(ctx, taskID)
	if err != nil {
		return nil, err
	}

	changed := map[string]any{}

	if title != nil {
		if *title == "" {
			return nil, errs.BadRequest("title cannot be empty")
		}
		changed["title"] = map[string]any{"from": task.Title, "to": *title}
		task.Title = *title
	}
	if description != nil {
		task.Description = *description
	}
	if priority != nil {
		if !validPriorities[*priority] {
			return nil, errs.BadRequest("priority must be one of: low, medium, high")
		}
		changed["priority"] = map[string]any{"from": task.Priority, "to": *priority}
		task.Priority = *priority
	}
	if assigneeID != nil {
		changed["assignee_id"] = map[string]any{"from": task.AssigneeID, "to": *assigneeID}
		task.AssigneeID = *assigneeID
	}
	if dueDate != nil {
		task.DueDate = *dueDate
	}

	task.UpdatedAt = time.Now()
	if err := s.repo.UpdateTask(ctx, task); err != nil {
		return nil, err
	}

	// Log using the actor stored on the task (created_by); UpdateTask has no
	// callerID param, so we use created_by as a reasonable actor proxy.
	s.logger.LogTask(ctx, task.CreatedBy, task.ID, ActionUpdated, changed)

	return task, nil
}

// MoveTask moves a task to a new stage at the given 0-based position.
// This is the drag-and-drop handler — position values in both the source
// and destination columns are kept contiguous by the repo layer.
func (s *TaskService) MoveTask(ctx context.Context, taskID, boardID, newStage string, newPosition int) (*model.Task, error) {
	if !validStages[newStage] {
		return nil, errs.BadRequest("stage must be one of: todo, in_progress, done")
	}
	if newPosition < 0 {
		return nil, errs.BadRequest("position must be >= 0")
	}

	// Read current stage before moving so we can include it in the log.
	before, err := s.repo.GetTaskByID(ctx, taskID)
	if err != nil {
		return nil, err
	}

	if err := s.repo.UpdateTaskStage(ctx, taskID, boardID, newStage, newPosition); err != nil {
		return nil, err
	}

	task, err := s.repo.GetTaskByID(ctx, taskID)
	if err != nil {
		return nil, err
	}

	s.logger.LogTask(ctx, before.CreatedBy, task.ID, ActionMoved, map[string]any{
		"from": before.Stage,
		"to":   newStage,
		"position": newPosition,
	})

	return task, nil
}
