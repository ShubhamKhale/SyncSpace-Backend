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

type createTaskRequest struct {
	Title       string  `json:"title"       binding:"required"`
	Description string  `json:"description"`
	Stage       string  `json:"stage"`    // default: todo
	Priority    string  `json:"priority"` // default: medium
	AssigneeID  string  `json:"assignee_id"`
	DueDate     *string `json:"due_date"` // RFC3339 string, optional
}

// updateTaskRequest uses pointer fields so PATCH only touches supplied fields.
type updateTaskRequest struct {
	Title       *string  `json:"title"`
	Description *string  `json:"description"`
	Priority    *string  `json:"priority"`
	AssigneeID  *string  `json:"assignee_id"`
	DueDate     *string  `json:"due_date"` // send null to clear, omit to keep
	ClearDueDate bool    `json:"clear_due_date"` // explicit clear flag
}

type moveTaskRequest struct {
	Stage    string `json:"stage"    binding:"required"`
	Position int    `json:"position" binding:"min=0"`
}

// ── Handlers ──────────────────────────────────────────────────────────────────

// GetTasks handles GET /api/boards/:id/tasks.
//
// Optional query filters:
//
//	?stage=todo|in_progress|done
//	?priority=low|medium|high
//	?assignee_id=<user-id>
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
	boardID   := c.Param("id")

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

	task, err := tc.svc.CreateTask(
		c.Request.Context(),
		callerID, boardID,
		req.Title, req.Description,
		req.Stage, req.Priority, req.AssigneeID,
		dueDate,
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
// Only the fields present in the JSON body are updated.
func (tc *TaskController) UpdateTask(c *gin.Context) {
	taskID := c.Param("id")

	var req updateTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, data.Fail(err.Error()))
		return
	}

	// Parse optional due_date — supports clearing via clear_due_date flag.
	var dueDatePtr **time.Time
	if req.ClearDueDate {
		var nilTime *time.Time // typed nil pointer signals "clear the value"
		dueDatePtr = &nilTime
	} else if req.DueDate != nil {
		parsed, err := time.Parse(time.RFC3339, *req.DueDate)
		if err != nil {
			c.JSON(http.StatusBadRequest, data.Fail("due_date must be RFC3339 format"))
			return
		}
		p := &parsed // *time.Time — one more address-of gives **time.Time
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
// Moves a task to a new stage column at the specified position (drag-and-drop).
// The board_id must be passed as a query parameter so the repo can correctly
// reorder sibling tasks within the same board.
//
//	PATCH /api/tasks/abc-123/stage?board_id=brd-456
//	{ "stage": "in_progress", "position": 2 }
func (tc *TaskController) MoveTask(c *gin.Context) {
	taskID  := c.Param("id")
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
