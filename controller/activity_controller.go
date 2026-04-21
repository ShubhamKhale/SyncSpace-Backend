package controller

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"syncspace-backend/constants"
	"syncspace-backend/pkg/utils"
	"syncspace-backend/service"
	"syncspace-backend/service/data"
)

// ActivityController handles activity log endpoints.
type ActivityController struct {
	svc *service.ActivityService
}

// NewActivityController creates an ActivityController backed by the given ActivityService.
func NewActivityController(svc *service.ActivityService) *ActivityController {
	return &ActivityController{svc: svc}
}

// GetActivityLogs handles GET /api/activity-logs.
//
// Query params:
//
//	?board_id=<id>   — filter to a specific board and its tasks (optional)
//	?user_id=<id>    — filter to a specific actor (optional; defaults to caller)
//	?limit=N         — page size (default 20, max 100)
//	?offset=N        — records to skip (default 0)
//
// At least one of board_id or user_id must be supplied. If neither is provided
// the endpoint defaults to the authenticated user's own activity.
//
// Response:
//
//	{ "logs": [...], "total": 42 }
//
// Each log entry includes actor_name (resolved by JOIN, no extra round-trip).
func (ac *ActivityController) GetActivityLogs(c *gin.Context) {
	callerID := mustUserID(c)

	boardID := c.Query("board_id")
	userID  := c.Query("user_id")

	// Default: show the authenticated user's own activity when no filter given.
	if boardID == "" && userID == "" {
		userID = callerID
	}

	q := service.ActivityQuery{
		BoardID: boardID,
		UserID:  userID,
		Limit:   parseQueryInt(c, "limit", 20),
		Offset:  parseQueryInt(c, "offset", 0),
	}

	page, err := ac.svc.GetLogs(c.Request.Context(), q)
	if err != nil {
		utils.Error(constants.LogTagActivity, "GetActivityLogs failed", err)
		c.JSON(appErrStatus(err), data.Fail(appErrMsg(err)))
		return
	}

	c.JSON(http.StatusOK, data.OK(page))
}
