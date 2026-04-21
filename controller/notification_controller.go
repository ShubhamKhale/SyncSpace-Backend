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

// NotificationController handles notification endpoints.
type NotificationController struct {
	svc *service.NotificationService
}

// NewNotificationController creates a NotificationController backed by the given service.
func NewNotificationController(svc *service.NotificationService) *NotificationController {
	return &NotificationController{svc: svc}
}

// GetNotifications handles GET /api/notifications.
//
// Optional query params:
//
//	?limit=N   — page size (default 20, max 100)
//	?offset=N  — number of records to skip (default 0)
//
// Response includes notifications for the current page plus the global
// unread_count and total (across all pages), computed in a single query.
func (nc *NotificationController) GetNotifications(c *gin.Context) {
	userID := mustUserID(c)
	limit  := parseQueryInt(c, "limit", 20)
	offset := parseQueryInt(c, "offset", 0)

	page, err := nc.svc.GetNotifications(c.Request.Context(), userID, limit, offset)
	if err != nil {
		utils.Error(constants.LogTagNotification, "GetNotifications failed for "+userID, err)
		c.JSON(appErrStatus(err), data.Fail(appErrMsg(err)))
		return
	}

	c.JSON(http.StatusOK, data.OK(page))
}

// MarkRead handles PATCH /api/notifications/:id/read.
// Marks a single notification as read. Only the owning user can mark their
// own notifications — ownership is enforced at the database WHERE clause.
func (nc *NotificationController) MarkRead(c *gin.Context) {
	userID  := mustUserID(c)
	notifID := c.Param("id")

	if err := nc.svc.MarkRead(c.Request.Context(), notifID, userID); err != nil {
		utils.Error(constants.LogTagNotification, "MarkRead failed notif="+notifID, err)
		c.JSON(appErrStatus(err), data.Fail(appErrMsg(err)))
		return
	}

	c.JSON(http.StatusOK, data.OK(gin.H{"id": notifID, "is_read": true}))
}

// MarkAllRead handles PATCH /api/notifications/mark-all-read.
// Marks every unread notification for the authenticated user as read in a
// single UPDATE statement. Returns the count of rows updated.
func (nc *NotificationController) MarkAllRead(c *gin.Context) {
	userID := mustUserID(c)

	updated, err := nc.svc.MarkAllRead(c.Request.Context(), userID)
	if err != nil {
		utils.Error(constants.LogTagNotification, "MarkAllRead failed for "+userID, err)
		c.JSON(appErrStatus(err), data.Fail(appErrMsg(err)))
		return
	}

	utils.Info(constants.LogTagNotification, "marked all read for user "+userID)
	c.JSON(http.StatusOK, data.OK(gin.H{"marked_read": updated}))
}

// parseQueryInt reads an integer query param, returning defaultVal on absence or parse error.
func parseQueryInt(c *gin.Context, key string, defaultVal int) int {
	if raw := c.Query(key); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil {
			return n
		}
	}
	return defaultVal
}
