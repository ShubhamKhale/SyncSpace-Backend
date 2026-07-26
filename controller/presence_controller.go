package controller

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"syncspace-backend/pkg/utils"
	"syncspace-backend/service"
	"syncspace-backend/service/data"
)

// PresenceController handles REST presence endpoints for flow diagram rooms.
type PresenceController struct {
	svc *service.PresenceService
}

// NewPresenceController creates a PresenceController.
func NewPresenceController(svc *service.PresenceService) *PresenceController {
	return &PresenceController{svc: svc}
}

func (pc *PresenceController) unavailable(c *gin.Context) bool {
	if pc.svc == nil {
		c.JSON(http.StatusServiceUnavailable, data.Fail("presence requires REDIS_URL"))
		return true
	}
	return false
}

// Join handles POST /api/boards/:id/flows/:flowId/participants/join.
func (pc *PresenceController) Join(c *gin.Context) {
	if pc.unavailable(c) {
		return
	}
	userID := mustUserID(c)
	flowID := c.Param("flowId")

	p, err := pc.svc.Join(c.Request.Context(), flowID, userID)
	if err != nil {
		utils.Error("[PRESENCE]", "Join failed flow="+flowID, err)
		c.JSON(appErrStatus(err), data.Fail(appErrMsg(err)))
		return
	}
	c.JSON(http.StatusOK, data.OK(p))
}

// List handles GET /api/boards/:id/flows/:flowId/participants.
func (pc *PresenceController) List(c *gin.Context) {
	if pc.unavailable(c) {
		return
	}
	flowID := c.Param("flowId")

	participants, err := pc.svc.ListParticipants(c.Request.Context(), flowID)
	if err != nil {
		utils.Error("[PRESENCE]", "List failed flow="+flowID, err)
		c.JSON(appErrStatus(err), data.Fail(appErrMsg(err)))
		return
	}
	c.JSON(http.StatusOK, data.OK(participants))
}

// Leave handles DELETE /api/boards/:id/flows/:flowId/participants/leave.
func (pc *PresenceController) Leave(c *gin.Context) {
	if pc.unavailable(c) {
		return
	}
	userID := mustUserID(c)
	flowID := c.Param("flowId")

	if err := pc.svc.Leave(c.Request.Context(), flowID, userID); err != nil {
		utils.Error("[PRESENCE]", "Leave failed flow="+flowID, err)
		c.JSON(appErrStatus(err), data.Fail(appErrMsg(err)))
		return
	}
	c.Status(http.StatusNoContent)
}
