package controller

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"syncspace-backend/constants"
	"syncspace-backend/pkg/utils"
	"syncspace-backend/service"
	"syncspace-backend/service/data"
)

// FlowController handles flow diagram endpoints.
type FlowController struct {
	svc *service.FlowService
}

// NewFlowController creates a FlowController backed by the given FlowService.
func NewFlowController(svc *service.FlowService) *FlowController {
	return &FlowController{svc: svc}
}

// ── Request types ─────────────────────────────────────────────────────────────

// updateFlowRequest is the PUT /api/flows/:id body.
// version must match the current stored version (optimistic locking).
type updateFlowRequest struct {
	Data    json.RawMessage `json:"data"    binding:"required"`
	Version int             `json:"version"`
}

// ── Handlers ──────────────────────────────────────────────────────────────────

// GetFlow handles GET /api/flows/:id.
// Returns the full flow including its JSONB data, version, and metadata.
func (fc *FlowController) GetFlow(c *gin.Context) {
	flowID := c.Param("id")

	flow, err := fc.svc.GetFlow(c.Request.Context(), flowID)
	if err != nil {
		utils.Error(constants.LogTagFlow, "GetFlow failed flow="+flowID, err)
		c.JSON(appErrStatus(err), data.Fail(appErrMsg(err)))
		return
	}

	c.JSON(http.StatusOK, data.OK(flow))
}

// UpdateFlow handles PUT /api/flows/:id.
// Replaces the flow's data payload with optimistic-locking version enforcement.
//
// Request body:
//
//	{
//	  "data":    { "nodes": [...], "edges": [...] },
//	  "version": 3
//	}
//
// Returns 409 Conflict when the provided version does not match the current
// stored version, signalling that another client has written concurrently.
// The caller should re-fetch (GET /api/flows/:id), merge their changes, and retry.
func (fc *FlowController) UpdateFlow(c *gin.Context) {
	callerID := mustUserID(c)
	flowID   := c.Param("id")

	var req updateFlowRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, data.Fail(err.Error()))
		return
	}

	flow, err := fc.svc.UpdateFlow(c.Request.Context(), flowID, callerID, req.Data, req.Version)
	if err != nil {
		utils.Error(constants.LogTagFlow, "UpdateFlow failed flow="+flowID, err)
		c.JSON(appErrStatus(err), data.Fail(appErrMsg(err)))
		return
	}

	utils.Info(constants.LogTagFlow, "flow updated: "+flowID+" v"+strconv.Itoa(flow.Version))
	c.JSON(http.StatusOK, data.OK(flow))
}
