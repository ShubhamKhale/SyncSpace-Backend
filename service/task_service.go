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

// SubtaskInput is the client-supplied shape for a subtask. ID is empty for new
// subtasks (one is generated); on update, an existing ID is preserved.
type SubtaskInput struct {
	ID        string
	Text      string
	Completed bool
}

// TaskPatch holds a partial task update. A nil field is left unchanged.
// For the date fields, a non-nil pointer to a nil *time.Time clears the date.
type TaskPatch struct {
	Title           *string
	Description     *string
	Priority        *string
	Stage           *string
	AssigneeID      *string // "" unassigns
	DueDate         **time.Time
	StartDate       **time.Time
	Tags            *[]string
	TimeEstimate    *string
	ReferenceLink   *string
	FlowDiagramLink *string
	Subtasks        *[]SubtaskInput
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

// requireBoardInOrg returns errs.NotFound unless the board belongs to orgID.
// NotFound (not Forbidden) so callers can't probe for other orgs' board IDs.
func (s *TaskService) requireBoardInOrg(ctx context.Context, boardID, orgID string) error {
	ok, err := s.repo.BoardInOrg(ctx, boardID, orgID)
	if err != nil {
		return err
	}
	if !ok {
		return errs.NotFound("board not found")
	}
	return nil
}

// getTaskInOrg loads a task, returning errs.NotFound unless its board belongs to orgID.
func (s *TaskService) getTaskInOrg(ctx context.Context, taskID, orgID string) (*model.Task, error) {
	taskOrg, err := s.repo.GetTaskOrgID(ctx, taskID)
	if err != nil {
		return nil, err
	}
	if taskOrg != orgID {
		return nil, errs.NotFound("task not found")
	}
	return s.repo.GetTaskByID(ctx, taskID)
}

// validateAssignee ensures a non-empty assignee is an active member of the org.
func (s *TaskService) validateAssignee(ctx context.Context, orgID, assigneeID string) error {
	if assigneeID == "" {
		return nil
	}
	ok, err := s.repo.IsActiveOrgMember(ctx, orgID, assigneeID)
	if err != nil {
		return err
	}
	if !ok {
		return errs.BadRequest("assignee must be an active member of the organization")
	}
	return nil
}

func validateTags(tags []string) error {
	if len(tags) > 10 {
		return errs.BadRequest("tags: max 10 items allowed")
	}
	for _, t := range tags {
		if len(t) > 50 {
			return errs.BadRequest("tags: each tag must be 50 characters or fewer")
		}
	}
	return nil
}

// buildSubtasks validates and converts subtask inputs, keeping existing IDs.
func buildSubtasks(in []SubtaskInput) ([]model.Subtask, error) {
	if len(in) > 50 {
		return nil, errs.BadRequest("subtasks: max 50 items allowed")
	}
	out := make([]model.Subtask, 0, len(in))
	for _, st := range in {
		if st.Text == "" {
			continue
		}
		id := st.ID
		if id == "" {
			id = utils.NewUUID()
		}
		out = append(out, model.Subtask{ID: id, Text: st.Text, Completed: st.Completed})
	}
	return out, nil
}

// CreateTask validates input and persists a new task on the given board.
func (s *TaskService) CreateTask(
	ctx context.Context,
	orgID, createdBy, boardID, title, description, stage, priority, assigneeID string,
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
	if err := s.requireBoardInOrg(ctx, boardID, orgID); err != nil {
		return nil, err
	}
	if err := s.validateAssignee(ctx, orgID, assigneeID); err != nil {
		return nil, err
	}
	if err := validateTags(tags); err != nil {
		return nil, err
	}
	modelSubtasks, err := buildSubtasks(subtasks)
	if err != nil {
		return nil, err
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
func (s *TaskService) GetTasks(ctx context.Context, orgID, boardID string, filter database.TaskFilter) ([]model.Task, error) {
	if filter.Stage != "" && !validStages[filter.Stage] {
		return nil, errs.BadRequest("stage must be one of: Planning, Design, Development, QA, Deployment")
	}
	if filter.Priority != "" && !validPriorities[filter.Priority] {
		return nil, errs.BadRequest("priority must be one of: low, medium, high")
	}
	if err := s.requireBoardInOrg(ctx, boardID, orgID); err != nil {
		return nil, err
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

// UpdateTask applies a partial update to a task in the caller's org.
// A stage change moves the task to the bottom of the new column.
func (s *TaskService) UpdateTask(ctx context.Context, orgID, callerID, taskID string, p TaskPatch) (*model.Task, error) {
	task, err := s.getTaskInOrg(ctx, taskID, orgID)
	if err != nil {
		return nil, err
	}

	changed := map[string]any{}
	diff := func(field string, from, to any) {
		changed[field] = map[string]any{"from": from, "to": to}
	}

	if p.Title != nil {
		if *p.Title == "" {
			return nil, errs.BadRequest("title cannot be empty")
		}
		if *p.Title != task.Title {
			diff("title", task.Title, *p.Title)
		}
		task.Title = *p.Title
	}
	if p.Description != nil {
		task.Description = *p.Description
	}
	if p.Priority != nil {
		if !validPriorities[*p.Priority] {
			return nil, errs.BadRequest("priority must be one of: low, medium, high")
		}
		if *p.Priority != task.Priority {
			diff("priority", task.Priority, *p.Priority)
		}
		task.Priority = *p.Priority
	}
	newStage := ""
	if p.Stage != nil && *p.Stage != task.Stage {
		if !validStages[*p.Stage] {
			return nil, errs.BadRequest("stage must be one of: Planning, Design, Development, QA, Deployment")
		}
		diff("stage", task.Stage, *p.Stage)
		newStage = *p.Stage
	}
	if p.AssigneeID != nil && *p.AssigneeID != task.AssigneeID {
		if err := s.validateAssignee(ctx, orgID, *p.AssigneeID); err != nil {
			return nil, err
		}
		diff("assignee_id", task.AssigneeID, *p.AssigneeID)
		task.AssigneeID = *p.AssigneeID
	}
	if p.DueDate != nil {
		task.DueDate = *p.DueDate
	}
	if p.StartDate != nil {
		task.StartDate = *p.StartDate
	}
	if task.StartDate != nil && task.DueDate != nil && task.DueDate.Before(*task.StartDate) {
		return nil, errs.BadRequest("due_date cannot be before start_date")
	}
	if p.Tags != nil {
		if err := validateTags(*p.Tags); err != nil {
			return nil, err
		}
		task.Tags = *p.Tags
	}
	if p.TimeEstimate != nil {
		task.TimeEstimate = *p.TimeEstimate
	}
	if p.ReferenceLink != nil {
		task.ReferenceLink = *p.ReferenceLink
	}
	if p.FlowDiagramLink != nil {
		task.FlowDiagramLink = *p.FlowDiagramLink
	}
	if p.Subtasks != nil {
		subs, err := buildSubtasks(*p.Subtasks)
		if err != nil {
			return nil, err
		}
		task.Subtasks = subs
	}

	task.UpdatedAt = time.Now()
	if err := s.repo.UpdateTask(ctx, task); err != nil {
		return nil, err
	}

	if newStage != "" {
		pos := s.repo.NextPosition(ctx, task.BoardID, newStage)
		if err := s.repo.UpdateTaskStage(ctx, task.ID, task.BoardID, newStage, pos); err != nil {
			return nil, err
		}
		task.Stage = newStage
		task.Position = pos
	}

	action := ActionUpdated
	if _, onlyAssignee := changed["assignee_id"]; onlyAssignee && len(changed) == 1 {
		action = ActionAssigned
	}
	s.logger.LogTask(ctx, callerID, task.ID, action, changed)

	return task, nil
}

// DeleteTask permanently removes a task in the caller's org.
func (s *TaskService) DeleteTask(ctx context.Context, orgID, callerID, taskID string) error {
	task, err := s.getTaskInOrg(ctx, taskID, orgID)
	if err != nil {
		return err
	}
	if err := s.repo.DeleteTask(ctx, taskID); err != nil {
		return err
	}
	s.logger.LogTask(ctx, callerID, taskID, ActionDeleted, map[string]any{
		"board_id": task.BoardID,
		"title":    task.Title,
		"stage":    task.Stage,
	})
	return nil
}

// MoveTask moves a task to a new stage at the given 0-based position.
func (s *TaskService) MoveTask(ctx context.Context, orgID, callerID, taskID, boardID, newStage string, newPosition int) (*model.Task, error) {
	if !validStages[newStage] {
		return nil, errs.BadRequest("stage must be one of: Planning, Design, Development, QA, Deployment")
	}
	if newPosition < 0 {
		return nil, errs.BadRequest("position must be >= 0")
	}

	before, err := s.getTaskInOrg(ctx, taskID, orgID)
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

	s.logger.LogTask(ctx, callerID, task.ID, ActionMoved, map[string]any{
		"from":     before.Stage,
		"to":       newStage,
		"position": newPosition,
	})

	return task, nil
}
