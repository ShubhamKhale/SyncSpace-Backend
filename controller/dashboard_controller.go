package controller

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"syncspace-backend/constants"
	"syncspace-backend/pkg/utils"
	"syncspace-backend/service"
	"syncspace-backend/service/data"
)

// DashboardController handles dashboard overview endpoints.
type DashboardController struct {
	svc *service.DashboardService
}

// NewDashboardController creates a DashboardController backed by the given DashboardService.
func NewDashboardController(svc *service.DashboardService) *DashboardController {
	return &DashboardController{svc: svc}
}

// GetStats handles GET /api/dashboard/stats.
// Returns aggregated board and task counts for the authenticated user.
func (dc *DashboardController) GetStats(c *gin.Context) {
	userID := mustUserID(c)

	stats, err := dc.svc.GetStats(c.Request.Context(), userID)
	if err != nil {
		utils.Error(constants.LogTagDashboard, "GetStats failed for "+userID, err)
		c.JSON(appErrStatus(err), data.Fail(appErrMsg(err)))
		return
	}

	c.JSON(http.StatusOK, data.OK(stats))
}

// GetRecentBoards handles GET /api/boards/recent.
// Optional query param: ?limit=N (default 5, max 20).
func (dc *DashboardController) GetRecentBoards(c *gin.Context) {
	userID := mustUserID(c)
	limit := parseLimit(c, 5)

	boards, err := dc.svc.GetRecentBoards(c.Request.Context(), userID, limit)
	if err != nil {
		utils.Error(constants.LogTagDashboard, "GetRecentBoards failed for "+userID, err)
		c.JSON(appErrStatus(err), data.Fail(appErrMsg(err)))
		return
	}

	c.JSON(http.StatusOK, data.OK(boards))
}

// GetUpcomingTasks handles GET /api/tasks/upcoming.
// Returns non-done tasks assigned to or created by the user, due within 7 days.
// Optional query param: ?limit=N (default 10, max 50).
func (dc *DashboardController) GetUpcomingTasks(c *gin.Context) {
	userID := mustUserID(c)
	limit := parseLimit(c, 10)

	tasks, err := dc.svc.GetUpcomingTasks(c.Request.Context(), userID, limit)
	if err != nil {
		utils.Error(constants.LogTagDashboard, "GetUpcomingTasks failed for "+userID, err)
		c.JSON(appErrStatus(err), data.Fail(appErrMsg(err)))
		return
	}

	c.JSON(http.StatusOK, data.OK(tasks))
}

// parseLimit reads the ?limit= query param and falls back to defaultVal if absent or invalid.
func parseLimit(c *gin.Context, defaultVal int) int {
	if raw := c.Query("limit"); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil && n > 0 {
			return n
		}
	}
	return defaultVal
}
