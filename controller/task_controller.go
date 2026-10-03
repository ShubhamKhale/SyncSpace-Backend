package controller

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"syncspace-backend/constants"
	"syncspace-backend/pkg/utils"
	"syncspace-backend/service"
	"syncspace-backend/service/data"
	"syncspace-backend/service/database"
)

// TaskController handles task endpoints.
type TaskController struct {
	svc *service.TaskService
}

// NewTaskController creates a TaskController backed by the given TaskService.
func NewTaskController(svc *service.TaskService) *TaskController {
	return &TaskController{svc: svc}
}

// ── Request types ─────────────────────────────────────────────────────────────

type subtaskInput struct {
	ID        string `json:"id"`
	Text      string `json:"text"`
	Completed bool   `json:"completed"`
}

func toServiceSubtasks(in []subtaskInput) []service.SubtaskInput {
	out := make([]service.SubtaskInput, len(in))
	for i, s := range in {
		out[i] = service.SubtaskInput{ID: s.ID, Text: s.Text, Completed: s.Completed}
	}
	return out
}

// taskWriteRoles may create, edit, move and delete tasks; viewers are read-only.
var taskWriteRoles = map[string]bool{"owner": true, "admin": true, "member": true}

// taskAccess resolves the caller's org (and, when write is true, checks their
// role may modify tasks). On failure it writes the error response and returns ok=false.
func taskAccess(c *gin.Context, write bool) (orgID string, ok bool) {
	oid, _ := c.Get(string(constants.ContextKeyOrgID))
	orgID, _ = oid.(string)
	if orgID == "" {
		c.JSON(http.StatusForbidden, data.Fail("you must belong to an organization"))
		return "", false
	}
	if write {
		r, _ := c.Get(string(constants.ContextKeyOrgRole))
		role, _ := r.(string)
		if !taskWriteRoles[role] {
			c.JSON(http.StatusForbidden, data.Fail("viewers cannot modify tasks"))
			return "", false
		}
	}
	return orgID, true
}

// parseOptionalDate parses an RFC3339 date pointer; nil in → nil out.
func parseOptionalDate(raw *string, field string) (*time.Time, error) {
	if raw == nil {
		return nil, nil
	}
	t, err := time.Parse(time.RFC3339, *raw)
	if err != nil {
		return nil, fmt.Errorf("%s must be RFC3339 format (e.g. 2026-01-15T09:00:00Z)", field)
	}
	return &t, nil
}

// datePatch turns (value, clear) request fields into the service's **time.Time
// convention: nil = untouched, &nil = cleared, &t = set.
func datePatch(raw *string, clear bool, field string) (**time.Time, error) {
	if clear {
		var none *time.Time
		return &none, nil
	}
	t, err := parseOptionalDate(raw, field)
	if err != nil || t == nil {
		return nil, err
	}
	return &t, nil
}

type attachmentInput struct {
	Name string `json:"name"`
	URL  string `json:"url"`
	Type string `json:"type"`
	Size int64  `json:"size"`
}

type createTaskRequest struct {
	Title           string            `json:"title"             binding:"required"`
	Description     string            `json:"description"`
	Stage           string            `json:"stage"`
	Priority        string            `json:"priority"`
	AssigneeID      string            `json:"assignee_id"`
	DueDate         *string           `json:"due_date"`
	StartDate       *string           `json:"start_date"`
	Tags            []string          `json:"tags"`
	TimeEstimate    string            `json:"time_estimate"`
	ReferenceLink   string            `json:"reference_link"`
	FlowDiagramLink string            `json:"flow_diagram_link"`
	Subtasks        []subtaskInput    `json:"subtasks"`
	Attachments     []attachmentInput `json:"attachments"`
}

// updateTaskRequest uses pointer fields so PATCH only touches supplied fields.
// assignee_id "" unassigns; clear_due_date / clear_start_date remove a date.
type updateTaskRequest struct {
	Title           *string         `json:"title"`
	Description     *string         `json:"description"`
	Priority        *string         `json:"priority"`
	Stage           *string         `json:"stage"`
	AssigneeID      *string         `json:"assignee_id"`
	DueDate         *string         `json:"due_date"`
	ClearDueDate    bool            `json:"clear_due_date"`
	StartDate       *string         `json:"start_date"`
	ClearStartDate  bool            `json:"clear_start_date"`
	Tags            *[]string       `json:"tags"`
	TimeEstimate    *string         `json:"time_estimate"`
	ReferenceLink   *string         `json:"reference_link"`
	FlowDiagramLink *string         `json:"flow_diagram_link"`
	Subtasks        *[]subtaskInput `json:"subtasks"`
}

type moveTaskRequest struct {
	Stage    string `json:"stage"    binding:"required"`
	Position int    `json:"position" binding:"min=0"`
}

// ── Handlers ──────────────────────────────────────────────────────────────────

// GetTasks handles GET /api/boards/:id/tasks.
func (tc *TaskController) GetTasks(c *gin.Context) {
	orgID, ok := taskAccess(c, false)
	if !ok {
		return
	}
	boardID := c.Param("id")

	filter := database.TaskFilter{
		Stage:      c.Query("stage"),
		Priority:   c.Query("priority"),
		AssigneeID: c.Query("assignee_id"),
	}

	tasks, err := tc.svc.GetTasks(c.Request.Context(), orgID, boardID, filter)
	if err != nil {
		utils.Error(constants.LogTagTask, "GetTasks failed board="+boardID, err)
		c.JSON(appErrStatus(err), data.Fail(appErrMsg(err)))
		return
	}

	c.JSON(http.StatusOK, data.OK(tasks))
}

// CreateTask handles POST /api/boards/:id/tasks.
func (tc *TaskController) CreateTask(c *gin.Context) {
	orgID, ok := taskAccess(c, true)
	if !ok {
		return
	}
	callerID := mustUserID(c)
	boardID := c.Param("id")

	var req createTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, data.Fail(err.Error()))
		return
	}

	dueDate, err := parseOptionalDate(req.DueDate, "due_date")
	if err != nil {
		c.JSON(http.StatusBadRequest, data.Fail(err.Error()))
		return
	}
	startDate, err := parseOptionalDate(req.StartDate, "start_date")
	if err != nil {
		c.JSON(http.StatusBadRequest, data.Fail(err.Error()))
		return
	}

	svcAttachments := make([]service.AttachmentInput, len(req.Attachments))
	for i, a := range req.Attachments {
		svcAttachments[i] = service.AttachmentInput{
			Name: a.Name,
			URL:  a.URL,
			Type: a.Type,
			Size: a.Size,
		}
	}

	task, err := tc.svc.CreateTask(
		c.Request.Context(),
		orgID, callerID, boardID,
		req.Title, req.Description,
		req.Stage, req.Priority, req.AssigneeID,
		dueDate, startDate,
		req.Tags,
		req.TimeEstimate, req.ReferenceLink, req.FlowDiagramLink,
		toServiceSubtasks(req.Subtasks), svcAttachments,
	)
	if err != nil {
		utils.Error(constants.LogTagTask, "CreateTask failed board="+boardID, err)
		c.JSON(appErrStatus(err), data.Fail(appErrMsg(err)))
		return
	}

	utils.Info(constants.LogTagTask, "task created: "+task.ID+" on board "+boardID)
	c.JSON(http.StatusCreated, data.OK(task))
}

// UpdateTask handles PATCH /api/tasks/:id.
func (tc *TaskController) UpdateTask(c *gin.Context) {
	orgID, ok := taskAccess(c, true)
	if !ok {
		return
	}
	taskID := c.Param("id")

	var req updateTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, data.Fail(err.Error()))
		return
	}

	dueDate, err := datePatch(req.DueDate, req.ClearDueDate, "due_date")
	if err != nil {
		c.JSON(http.StatusBadRequest, data.Fail(err.Error()))
		return
	}
	startDate, err := datePatch(req.StartDate, req.ClearStartDate, "start_date")
	if err != nil {
		c.JSON(http.StatusBadRequest, data.Fail(err.Error()))
		return
	}

	patch := service.TaskPatch{
		Title:           req.Title,
		Description:     req.Description,
		Priority:        req.Priority,
		Stage:           req.Stage,
		AssigneeID:      req.AssigneeID,
		DueDate:         dueDate,
		StartDate:       startDate,
		Tags:            req.Tags,
		TimeEstimate:    req.TimeEstimate,
		ReferenceLink:   req.ReferenceLink,
		FlowDiagramLink: req.FlowDiagramLink,
	}
	if req.Subtasks != nil {
		subs := toServiceSubtasks(*req.Subtasks)
		patch.Subtasks = &subs
	}

	task, err := tc.svc.UpdateTask(c.Request.Context(), orgID, mustUserID(c), taskID, patch)
	if err != nil {
		utils.Error(constants.LogTagTask, "UpdateTask failed task="+taskID, err)
		c.JSON(appErrStatus(err), data.Fail(appErrMsg(err)))
		return
	}

	utils.Info(constants.LogTagTask, "task updated: "+taskID)
	c.JSON(http.StatusOK, data.OK(task))
}

// DeleteTask handles DELETE /api/tasks/:id.
func (tc *TaskController) DeleteTask(c *gin.Context) {
	orgID, ok := taskAccess(c, true)
	if !ok {
		return
	}
	taskID := c.Param("id")

	if err := tc.svc.DeleteTask(c.Request.Context(), orgID, mustUserID(c), taskID); err != nil {
		utils.Error(constants.LogTagTask, "DeleteTask failed task="+taskID, err)
		c.JSON(appErrStatus(err), data.Fail(appErrMsg(err)))
		return
	}

	utils.Info(constants.LogTagTask, "task deleted: "+taskID)
	c.JSON(http.StatusOK, data.OK(gin.H{"id": taskID}))
}

// MoveTask handles PATCH /api/tasks/:id/stage.
func (tc *TaskController) MoveTask(c *gin.Context) {
	orgID, ok := taskAccess(c, true)
	if !ok {
		return
	}
	taskID := c.Param("id")
	boardID := c.Query("board_id")
	if boardID == "" {
		c.JSON(http.StatusBadRequest, data.Fail("board_id query parameter is required"))
		return
	}

	var req moveTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, data.Fail(err.Error()))
		return
	}

	task, err := tc.svc.MoveTask(c.Request.Context(), orgID, mustUserID(c), taskID, boardID, req.Stage, req.Position)
	if err != nil {
		utils.Error(constants.LogTagTask, "MoveTask failed task="+taskID, err)
		c.JSON(appErrStatus(err), data.Fail(appErrMsg(err)))
		return
	}

	utils.Info(constants.LogTagTask, "task moved: "+taskID+" → stage="+req.Stage)
	c.JSON(http.StatusOK, data.OK(task))
}
