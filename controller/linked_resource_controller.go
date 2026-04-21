package controller

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"syncspace-backend/constants"
	"syncspace-backend/pkg/utils"
	"syncspace-backend/service"
	"syncspace-backend/service/data"
)

// LinkedResourceController handles board-level linked resource endpoints.
type LinkedResourceController struct {
	svc *service.LinkedResourceService
}

// NewLinkedResourceController creates a LinkedResourceController backed by the given service.
func NewLinkedResourceController(svc *service.LinkedResourceService) *LinkedResourceController {
	return &LinkedResourceController{svc: svc}
}

// ── Request types ─────────────────────────────────────────────────────────────

// linkedResourceItem is one entry in the PUT request body.
type linkedResourceItem struct {
	Label string `json:"label"`            // optional display name
	URL   string `json:"url" binding:"required"`
}

// replaceLinkedResourcesRequest is the PUT /api/boards/:id/linked-resources body.
// An empty resources array is valid — it clears all links for the board.
type replaceLinkedResourcesRequest struct {
	Resources []linkedResourceItem `json:"resources" binding:"required"`
}

// ── Handlers ──────────────────────────────────────────────────────────────────

// GetLinkedResources handles GET /api/boards/:id/linked-resources.
// Returns all linked resources attached to the board, ordered by creation time.
func (lc *LinkedResourceController) GetLinkedResources(c *gin.Context) {
	boardID := c.Param("id")

	resources, err := lc.svc.GetByBoard(c.Request.Context(), boardID)
	if err != nil {
		utils.Error(constants.LogTagLinkedResource, "GetLinkedResources failed board="+boardID, err)
		c.JSON(appErrStatus(err), data.Fail(appErrMsg(err)))
		return
	}

	c.JSON(http.StatusOK, data.OK(resources))
}

// ReplaceLinkedResources handles PUT /api/boards/:id/linked-resources.
// Atomically replaces the complete list of linked resources for the board.
//
// Request body:
//
//	{ "resources": [{ "label": "Figma", "url": "https://..." }, ...] }
//
// Sending an empty array clears all resources:
//
//	{ "resources": [] }
//
// Response: the newly persisted list with server-assigned IDs and timestamps.
func (lc *LinkedResourceController) ReplaceLinkedResources(c *gin.Context) {
	callerID := mustUserID(c)
	boardID  := c.Param("id")

	var req replaceLinkedResourcesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, data.Fail(err.Error()))
		return
	}

	// Map controller DTOs to service inputs.
	inputs := make([]service.LinkedResourceInput, len(req.Resources))
	for i, item := range req.Resources {
		inputs[i] = service.LinkedResourceInput{
			Label: item.Label,
			URL:   item.URL,
		}
	}

	resources, err := lc.svc.ReplaceAll(c.Request.Context(), boardID, callerID, inputs)
	if err != nil {
		utils.Error(constants.LogTagLinkedResource, "ReplaceLinkedResources failed board="+boardID, err)
		c.JSON(appErrStatus(err), data.Fail(appErrMsg(err)))
		return
	}

	utils.Info(constants.LogTagLinkedResource, "linked resources replaced for board "+boardID)
	c.JSON(http.StatusOK, data.OK(resources))
}
