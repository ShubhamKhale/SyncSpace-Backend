package controller

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"syncspace-backend/constants"
	"syncspace-backend/pkg/utils"
	"syncspace-backend/service"
	"syncspace-backend/service/data"
)

// AnalyticsController handles the analytics endpoints.
// All handlers are read-only and require JWT authentication.
type AnalyticsController struct {
	svc *service.AnalyticsService
}

// NewAnalyticsController creates an AnalyticsController backed by the given service.
func NewAnalyticsController(svc *service.AnalyticsService) *AnalyticsController {
	return &AnalyticsController{svc: svc}
}

// GetTaskCompletionTrend handles GET /api/analytics/task-completion.
//
// Query params:
//
//	?period=week|month|quarter  (default: week)
//	?board_id=<id>              (default: all owned boards)
func (a *AnalyticsController) GetTaskCompletionTrend(c *gin.Context) {
	userID  := mustUserID(c)
	period  := validPeriod(c.Query("period"))
	boardID := c.Query("board_id")

	points, err := a.svc.GetTaskCompletionTrend(c.Request.Context(), userID, period, boardID)
	if err != nil {
		utils.Error(constants.LogTagAnalytics, "GetTaskCompletionTrend failed for "+userID, err)
		c.JSON(appErrStatus(err), data.Fail(appErrMsg(err)))
		return
	}

	c.JSON(http.StatusOK, data.OK(points))
}

// GetTaskDistribution handles GET /api/analytics/task-distribution.
//
// Query params:
//
//	?board_id=<id>  (default: all owned boards)
func (a *AnalyticsController) GetTaskDistribution(c *gin.Context) {
	userID  := mustUserID(c)
	boardID := c.Query("board_id")

	dist, err := a.svc.GetTaskDistribution(c.Request.Context(), userID, boardID)
	if err != nil {
		utils.Error(constants.LogTagAnalytics, "GetTaskDistribution failed for "+userID, err)
		c.JSON(appErrStatus(err), data.Fail(appErrMsg(err)))
		return
	}

	c.JSON(http.StatusOK, data.OK(dist))
}

// GetBoardActivity handles GET /api/analytics/board-activity.
//
// Query params:
//
//	?period=week|month|quarter  (default: week)
//	?board_id=<id>              (default: all owned boards)
func (a *AnalyticsController) GetBoardActivity(c *gin.Context) {
	userID  := mustUserID(c)
	period  := validPeriod(c.Query("period"))
	boardID := c.Query("board_id")

	points, err := a.svc.GetBoardActivity(c.Request.Context(), userID, period, boardID)
	if err != nil {
		utils.Error(constants.LogTagAnalytics, "GetBoardActivity failed for "+userID, err)
		c.JSON(appErrStatus(err), data.Fail(appErrMsg(err)))
		return
	}

	c.JSON(http.StatusOK, data.OK(points))
}

// GetTeamContribution handles GET /api/analytics/team-contribution.
//
// Query params:
//
//	?board_id=<id>  (default: all owned boards)
func (a *AnalyticsController) GetTeamContribution(c *gin.Context) {
	userID  := mustUserID(c)
	boardID := c.Query("board_id")

	contribs, err := a.svc.GetTeamContribution(c.Request.Context(), userID, boardID)
	if err != nil {
		utils.Error(constants.LogTagAnalytics, "GetTeamContribution failed for "+userID, err)
		c.JSON(appErrStatus(err), data.Fail(appErrMsg(err)))
		return
	}

	c.JSON(http.StatusOK, data.OK(contribs))
}

// validPeriod normalises the ?period= query param.
// Accepted values are "week", "month", "quarter"; anything else defaults to "week".
func validPeriod(p string) string {
	switch p {
	case "month", "quarter":
		return p
	default:
		return "week"
	}
}
