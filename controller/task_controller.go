package controller

import (
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
	Text      string `json:"text"`
	Completed bool   `json:"completed"`
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
type updateTaskRequest struct {
	Title        *string `json:"title"`
	Description  *string `json:"description"`
	Priority     *string `json:"priority"`
	AssigneeID   *string `json:"assignee_id"`
	DueDate      *string `json:"due_date"`
	ClearDueDate bool    `json:"clear_due_date"`
}

type moveTaskRequest struct {
	Stage    string `json:"stage"    binding:"required"`
	Position int    `json:"position" binding:"min=0"`
}

// ── Handlers ──────────────────────────────────────────────────────────────────

// GetTasks handles GET /api/boards/:id/tasks.
func (tc *TaskController) GetTasks(c *gin.Context) {
	boardID := c.Param("id")

	filter := database.TaskFilter{
		Stage:      c.Query("stage"),
		Priority:   c.Query("priority"),
		AssigneeID: c.Query("assignee_id"),
	}

	tasks, err := tc.svc.GetTasks(c.Request.Context(), boardID, filter)
	if err != nil {
		utils.Error(constants.LogTagTask, "GetTasks failed board="+boardID, err)
		c.JSON(appErrStatus(err), data.Fail(appErrMsg(err)))
		return
	}

	c.JSON(http.StatusOK, data.OK(tasks))
}

// CreateTask handles POST /api/boards/:id/tasks.
func (tc *TaskController) CreateTask(c *gin.Context) {
	callerID := mustUserID(c)
	boardID := c.Param("id")

	var req createTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, data.Fail(err.Error()))
		return
	}

	var dueDate *time.Time
	if req.DueDate != nil {
		parsed, err := time.Parse(time.RFC3339, *req.DueDate)
		if err != nil {
			c.JSON(http.StatusBadRequest, data.Fail("due_date must be RFC3339 format (e.g. 2026-01-15T09:00:00Z)"))
			return
		}
		dueDate = &parsed
	}

	var startDate *time.Time
	if req.StartDate != nil {
		parsed, err := time.Parse(time.RFC3339, *req.StartDate)
		if err != nil {
			c.JSON(http.StatusBadRequest, data.Fail("start_date must be RFC3339 format (e.g. 2026-01-15T09:00:00Z)"))
			return
		}
		startDate = &parsed
	}

	// Convert local input types to service input types
	svcSubtasks := make([]service.SubtaskInput, len(req.Subtasks))
	for i, s := range req.Subtasks {
		svcSubtasks[i] = service.SubtaskInput{Text: s.Text, Completed: s.Completed}
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
		callerID, boardID,
		req.Title, req.Description,
		req.Stage, req.Priority, req.AssigneeID,
		dueDate, startDate,
		req.Tags,
		req.TimeEstimate, req.ReferenceLink, req.FlowDiagramLink,
		svcSubtasks, svcAttachments,
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
	taskID := c.Param("id")

	var req updateTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, data.Fail(err.Error()))
		return
	}

	var dueDatePtr **time.Time
	if req.ClearDueDate {
		var nilTime *time.Time
		dueDatePtr = &nilTime
	} else if req.DueDate != nil {
		parsed, err := time.Parse(time.RFC3339, *req.DueDate)
		if err != nil {
			c.JSON(http.StatusBadRequest, data.Fail("due_date must be RFC3339 format"))
			return
		}
		p := &parsed
		dueDatePtr = &p
	}

	task, err := tc.svc.UpdateTask(
		c.Request.Context(),
		taskID,
		req.Title, req.Description, req.Priority, req.AssigneeID,
		dueDatePtr,
	)
	if err != nil {
		utils.Error(constants.LogTagTask, "UpdateTask failed task="+taskID, err)
		c.JSON(appErrStatus(err), data.Fail(appErrMsg(err)))
		return
	}

	utils.Info(constants.LogTagTask, "task updated: "+taskID)
	c.JSON(http.StatusOK, data.OK(task))
}

// MoveTask handles PATCH /api/tasks/:id/stage.
func (tc *TaskController) MoveTask(c *gin.Context) {
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

	task, err := tc.svc.MoveTask(c.Request.Context(), taskID, boardID, req.Stage, req.Position)
	if err != nil {
		utils.Error(constants.LogTagTask, "MoveTask failed task="+taskID, err)
		c.JSON(appErrStatus(err), data.Fail(appErrMsg(err)))
		return
	}

	utils.Info(constants.LogTagTask, "task moved: "+taskID+" → stage="+req.Stage)
	c.JSON(http.StatusOK, data.OK(task))
}
