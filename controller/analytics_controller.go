package controller

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"

	"syncspace-backend/constants"
	"syncspace-backend/pkg/utils"
	"syncspace-backend/service"
	"syncspace-backend/service/data"
)

// AnalyticsController handles the analytics endpoints.
type AnalyticsController struct {
	svc *service.AnalyticsService
}

// NewAnalyticsController creates an AnalyticsController backed by the given service.
func NewAnalyticsController(svc *service.AnalyticsService) *AnalyticsController {
	return &AnalyticsController{svc: svc}
}

// ── Existing endpoints (kept) ─────────────────────────────────────────────────

// GetTaskCompletionTrend handles GET /api/analytics/task-completion (daily, period-based).
func (a *AnalyticsController) GetTaskCompletionTrend(c *gin.Context) {
	orgID, _ := c.Get(string(constants.ContextKeyOrgID))
	orgIDStr, _ := orgID.(string)
	period := validPeriod(c.Query("period"))
	boardID := c.Query("board_id")

	points, err := a.svc.GetTaskCompletionTrend(c.Request.Context(), orgIDStr, period, boardID)
	if err != nil {
		utils.Error(constants.LogTagAnalytics, "GetTaskCompletionTrend failed", err)
		c.JSON(appErrStatus(err), data.Fail(appErrMsg(err)))
		return
	}
	c.JSON(http.StatusOK, data.OK(points))
}

// ── New dashboard analytics endpoints ────────────────────────────────────────

// GetTaskCompletionTrendMonthly handles GET /api/analytics/task-completion-trend.
// Returns last 7 months with created + completed counts.
func (a *AnalyticsController) GetTaskCompletionTrendMonthly(c *gin.Context) {
	orgID, _ := c.Get(string(constants.ContextKeyOrgID))
	orgIDStr, _ := orgID.(string)
	if orgIDStr == "" {
		c.JSON(http.StatusForbidden, data.Fail("you must belong to an organization"))
		return
	}

	points, err := a.svc.GetMonthlyTaskTrend(c.Request.Context(), orgIDStr)
	if err != nil {
		utils.Error(constants.LogTagAnalytics, "GetTaskCompletionTrendMonthly failed", err)
		c.JSON(appErrStatus(err), data.Fail(appErrMsg(err)))
		return
	}
	c.JSON(http.StatusOK, data.OK(points))
}

// GetTaskDistribution handles GET /api/analytics/task-distribution.
// Returns task counts grouped by stage as {name, value} pairs.
func (a *AnalyticsController) GetTaskDistribution(c *gin.Context) {
	orgID, _ := c.Get(string(constants.ContextKeyOrgID))
	orgIDStr, _ := orgID.(string)
	if orgIDStr == "" {
		c.JSON(http.StatusForbidden, data.Fail("you must belong to an organization"))
		return
	}

	dist, err := a.svc.GetTaskDistributionByStage(c.Request.Context(), orgIDStr)
	if err != nil {
		utils.Error(constants.LogTagAnalytics, "GetTaskDistribution failed", err)
		c.JSON(appErrStatus(err), data.Fail(appErrMsg(err)))
		return
	}
	c.JSON(http.StatusOK, data.OK(dist))
}

// GetBoardActivity handles GET /api/analytics/board-activity.
// Returns top 5 boards with edits/comments/shares counts.
func (a *AnalyticsController) GetBoardActivity(c *gin.Context) {
	orgID, _ := c.Get(string(constants.ContextKeyOrgID))
	orgIDStr, _ := orgID.(string)
	if orgIDStr == "" {
		c.JSON(http.StatusForbidden, data.Fail("you must belong to an organization"))
		return
	}

	items, err := a.svc.GetBoardActivityStats(c.Request.Context(), orgIDStr)
	if err != nil {
		utils.Error(constants.LogTagAnalytics, "GetBoardActivity failed", err)
		c.JSON(appErrStatus(err), data.Fail(appErrMsg(err)))
		return
	}
	c.JSON(http.StatusOK, data.OK(items))
}

// GetTeamContribution handles GET /api/analytics/team-contribution.
// Returns radar-chart data with dynamic member keys.
func (a *AnalyticsController) GetTeamContribution(c *gin.Context) {
	orgID, _ := c.Get(string(constants.ContextKeyOrgID))
	orgIDStr, _ := orgID.(string)
	if orgIDStr == "" {
		c.JSON(http.StatusForbidden, data.Fail("you must belong to an organization"))
		return
	}

	result, err := a.svc.GetTeamContributionByPhase(c.Request.Context(), orgIDStr)
	if err != nil {
		utils.Error(constants.LogTagAnalytics, "GetTeamContribution failed", err)
		c.JSON(appErrStatus(err), data.Fail(appErrMsg(err)))
		return
	}
	c.JSON(http.StatusOK, data.OK(result))
}

// GetDashboardAnalytics handles GET /api/analytics/dashboard.
// Returns all 4 datasets in a single response.
func (a *AnalyticsController) GetDashboardAnalytics(c *gin.Context) {
	orgID, _ := c.Get(string(constants.ContextKeyOrgID))
	orgIDStr, _ := orgID.(string)

	fmt.Println("orgIDStr:", orgIDStr) // Debug log to check orgIDStr value

	if orgIDStr == "" {
		c.JSON(http.StatusForbidden, data.Fail("you must belong to an organization"))
		return
	}

	dashboard, err := a.svc.GetDashboardAnalytics(c.Request.Context(), orgIDStr)
	if err != nil {
		fmt.Println("Error fetching dashboard analytics:", err.Error()) // Debug log to check error details
		utils.Error(constants.LogTagAnalytics, "GetDashboardAnalytics failed", err)
		c.JSON(appErrStatus(err), data.Fail(appErrMsg(err)))
		return
	}
	c.JSON(http.StatusOK, data.OK(dashboard))
}

// validPeriod normalises the ?period= query param.
func validPeriod(p string) string {
	switch p {
	case "month", "quarter":
		return p
	default:
		return "week"
	}
}
