package controller

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"syncspace-backend/constants"
	"syncspace-backend/pkg/utils"
	"syncspace-backend/service"
	"syncspace-backend/service/data"
)

// BoardController handles HTTP requests for the /boards resource.
type BoardController struct {
	svc *service.BoardService
}

// NewBoardController creates a BoardController wired to the provided service.
func NewBoardController(svc *service.BoardService) *BoardController {
	return &BoardController{svc: svc}
}

type createBoardRequest struct {
	Title       string `json:"title"       binding:"required"`
	Description string `json:"description"`
}

// CreateBoard handles POST /api/boards.
func (bc *BoardController) CreateBoard(c *gin.Context) {
	ownerID := mustUserID(c)
	orgID, _ := c.Get(string(constants.ContextKeyOrgID))
	orgIDStr, _ := orgID.(string)
	if orgIDStr == "" {
		c.JSON(http.StatusForbidden, data.Fail("you must belong to an organization to create boards"))
		return
	}

	var req createBoardRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(constants.LogTagBoard, "invalid request body", err)
		c.JSON(http.StatusBadRequest, data.Fail("invalid request body: "+err.Error()))
		return
	}

	board, err := bc.svc.CreateBoard(c.Request.Context(), ownerID, orgIDStr, req.Title, req.Description)
	if err != nil {
		utils.Error(constants.LogTagBoard, "CreateBoard failed", err)
		c.JSON(appErrStatus(err), data.Fail(appErrMsg(err)))
		return
	}

	utils.Info(constants.LogTagBoard, "board created: "+board.ID)
	c.JSON(http.StatusCreated, data.OK(board))
}

// GetBoards handles GET /api/boards.
func (bc *BoardController) GetBoards(c *gin.Context) {
	orgID, _ := c.Get(string(constants.ContextKeyOrgID))
	orgIDStr, _ := orgID.(string)
	if orgIDStr == "" {
		c.JSON(http.StatusForbidden, data.Fail("you must belong to an organization to view boards"))
		return
	}

	boards, err := bc.svc.GetBoardsByOrg(c.Request.Context(), orgIDStr)
	if err != nil {
		utils.Error(constants.LogTagBoard, "GetBoards failed", err)
		c.JSON(appErrStatus(err), data.Fail(appErrMsg(err)))
		return
	}

	c.JSON(http.StatusOK, data.OK(boards))
}

// GetBoardByID handles GET /api/boards/:id.
func (bc *BoardController) GetBoardByID(c *gin.Context) {
	orgID, _ := c.Get(string(constants.ContextKeyOrgID))
	orgIDStr, _ := orgID.(string)
	if orgIDStr == "" {
		c.JSON(http.StatusForbidden, data.Fail("you must belong to an organization"))
		return
	}

	boardID := c.Param("id")
	board, err := bc.svc.GetBoard(c.Request.Context(), boardID, orgIDStr)
	if err != nil {
		utils.Error(constants.LogTagBoard, "GetBoardByID failed board="+boardID, err)
		c.JSON(appErrStatus(err), data.Fail(appErrMsg(err)))
		return
	}

	c.JSON(http.StatusOK, data.OK(board))
}

// UpdateBoard handles PATCH /api/boards/:id.
// Only fields present in the JSON body are updated.
func (bc *BoardController) UpdateBoard(c *gin.Context) {
	orgID, _ := c.Get(string(constants.ContextKeyOrgID))
	orgIDStr, _ := orgID.(string)
	if orgIDStr == "" {
		c.JSON(http.StatusForbidden, data.Fail("you must belong to an organization"))
		return
	}

	boardID := c.Param("id")

	var req struct {
		Title       *string `json:"title"`
		Description *string `json:"description"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, data.Fail(err.Error()))
		return
	}

	if req.Title == nil && req.Description == nil {
		c.JSON(http.StatusBadRequest, data.Fail("provide at least one field to update"))
		return
	}

	board, err := bc.svc.UpdateBoard(c.Request.Context(), boardID, orgIDStr, req.Title, req.Description)
	if err != nil {
		utils.Error(constants.LogTagBoard, "UpdateBoard failed board="+boardID, err)
		c.JSON(appErrStatus(err), data.Fail(appErrMsg(err)))
		return
	}

	utils.Info(constants.LogTagBoard, "board updated: "+boardID)
	c.JSON(http.StatusOK, data.OK(board))
}

// GetBoardMembers handles GET /api/boards/:id/members.
func (bc *BoardController) GetBoardMembers(c *gin.Context) {
	orgID, _ := c.Get(string(constants.ContextKeyOrgID))
	orgIDStr, _ := orgID.(string)
	if orgIDStr == "" {
		c.JSON(http.StatusForbidden, data.Fail("you must belong to an organization"))
		return
	}

	boardID := c.Param("id")
	members, err := bc.svc.GetBoardMembers(c.Request.Context(), boardID, orgIDStr)
	if err != nil {
		utils.Error(constants.LogTagBoard, "GetBoardMembers failed board="+boardID, err)
		c.JSON(appErrStatus(err), data.Fail(appErrMsg(err)))
		return
	}

	c.JSON(http.StatusOK, data.OK(members))
}

// GetBoardHealth handles GET /api/boards/:id/health.
func (bc *BoardController) GetBoardHealth(c *gin.Context) {
	orgID, _ := c.Get(string(constants.ContextKeyOrgID))
	orgIDStr, _ := orgID.(string)
	if orgIDStr == "" {
		c.JSON(http.StatusForbidden, data.Fail("you must belong to an organization"))
		return
	}

	boardID := c.Param("id")
	health, err := bc.svc.GetBoardHealth(c.Request.Context(), boardID, orgIDStr)
	if err != nil {
		utils.Error(constants.LogTagBoard, "GetBoardHealth failed", err)
		c.JSON(appErrStatus(err), data.Fail(appErrMsg(err)))
		return
	}

	c.JSON(http.StatusOK, data.OK(health))
}

// GetBoardActivity handles GET /api/boards/:id/activity.
//
// Query params:
//
//	?limit=N — page size (default 10, max 50)
func (bc *BoardController) GetBoardActivity(c *gin.Context) {
	orgID, _ := c.Get(string(constants.ContextKeyOrgID))
	orgIDStr, _ := orgID.(string)
	if orgIDStr == "" {
		c.JSON(http.StatusForbidden, data.Fail("you must belong to an organization"))
		return
	}

	boardID := c.Param("id")
	limit := parseQueryInt(c, "limit", 10)

	entries, err := bc.svc.GetBoardActivity(c.Request.Context(), boardID, orgIDStr, limit)
	if err != nil {
		utils.Error(constants.LogTagBoard, "GetBoardActivity failed", err)
		c.JSON(appErrStatus(err), data.Fail(appErrMsg(err)))
		return
	}

	c.JSON(http.StatusOK, data.OK(entries))
}
