package service

import (
	"context"
	"time"

	"syncspace-backend/errs"
	"syncspace-backend/model"
	"syncspace-backend/pkg/utils"
	"syncspace-backend/service/database"
)

var validStages = map[string]bool{
	"Planning":    true,
	"Design":      true,
	"Development": true,
	"QA":          true,
	"Deployment":  true,
}

var validPriorities = map[string]bool{
	"low":    true,
	"medium": true,
	"high":   true,
}

// SubtaskInput is the create-time shape for a subtask (no ID yet).
type SubtaskInput struct {
	Text      string
	Completed bool
}

// AttachmentInput is the create-time shape for an attachment (no ID/timestamp yet).
type AttachmentInput struct {
	Name string
	URL  string
	Type string
	Size int64
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
func (s *TaskService) CreateTask(
	ctx context.Context,
	createdBy, boardID, title, description, stage, priority, assigneeID string,
	dueDate *time.Time,
	startDate *time.Time,
	tags []string,
	timeEstimate, referenceLink, flowDiagramLink string,
	subtasks []SubtaskInput,
	attachments []AttachmentInput,
) (*model.Task, error) {
	if title == "" {
		return nil, errs.BadRequest("title is required")
	}
	if stage == "" {
		stage = "Planning"
	}
	if priority == "" {
		priority = "medium"
	}
	if !validStages[stage] {
		return nil, errs.BadRequest("stage must be one of: Planning, Design, Development, QA, Deployment")
	}
	if !validPriorities[priority] {
		return nil, errs.BadRequest("priority must be one of: low, medium, high")
	}

	// Tags validation
	if len(tags) > 10 {
		return nil, errs.BadRequest("tags: max 10 items allowed")
	}
	for _, t := range tags {
		if len(t) > 50 {
			return nil, errs.BadRequest("tags: each tag must be 50 characters or fewer")
		}
	}

	// Subtasks validation
	if len(subtasks) > 50 {
		return nil, errs.BadRequest("subtasks: max 50 items allowed")
	}

	// Attachments validation
	if len(attachments) > 10 {
		return nil, errs.BadRequest("attachments: max 10 items allowed")
	}
	for _, a := range attachments {
		if a.URL == "" {
			return nil, errs.BadRequest("attachments: url is required for each attachment")
		}
		if a.Name == "" {
			return nil, errs.BadRequest("attachments: name is required for each attachment")
		}
	}

	// Normalize nil slices → empty (consistent JSON output)
	if tags == nil {
		tags = []string{}
	}

	now := time.Now()

	// Build subtasks with generated IDs
	modelSubtasks := make([]model.Subtask, len(subtasks))
	for i, s := range subtasks {
		modelSubtasks[i] = model.Subtask{
			ID:        utils.NewUUID(),
			Text:      s.Text,
			Completed: s.Completed,
		}
	}

	// Build attachments with generated IDs
	modelAttachments := make([]model.Attachment, len(attachments))
	for i, a := range attachments {
		modelAttachments[i] = model.Attachment{
			ID:         utils.NewUUID(),
			Name:       a.Name,
			URL:        a.URL,
			Type:       a.Type,
			Size:       a.Size,
			UploadedAt: now,
		}
	}

	task := &model.Task{
		ID:              utils.NewUUID(),
		BoardID:         boardID,
		Title:           title,
		Description:     description,
		Stage:           stage,
		Priority:        priority,
		AssigneeID:      assigneeID,
		CreatedBy:       createdBy,
		DueDate:         dueDate,
		StartDate:       startDate,
		Tags:            tags,
		TimeEstimate:    timeEstimate,
		ReferenceLink:   referenceLink,
		FlowDiagramLink: flowDiagramLink,
		Subtasks:        modelSubtasks,
		Attachments:     modelAttachments,
		CreatedAt:       now,
		UpdatedAt:       now,
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
func (s *TaskService) GetTasks(ctx context.Context, boardID string, filter database.TaskFilter) ([]model.Task, error) {
	if filter.Stage != "" && !validStages[filter.Stage] {
		return nil, errs.BadRequest("stage must be one of: Planning, Design, Development, QA, Deployment")
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

	s.logger.LogTask(ctx, task.CreatedBy, task.ID, ActionUpdated, changed)

	return task, nil
}

// MoveTask moves a task to a new stage at the given 0-based position.
func (s *TaskService) MoveTask(ctx context.Context, taskID, boardID, newStage string, newPosition int) (*model.Task, error) {
	if !validStages[newStage] {
		return nil, errs.BadRequest("stage must be one of: Planning, Design, Development, QA, Deployment")
	}
	if newPosition < 0 {
		return nil, errs.BadRequest("position must be >= 0")
	}

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
		"from":     before.Stage,
		"to":       newStage,
		"position": newPosition,
	})

	return task, nil
}
