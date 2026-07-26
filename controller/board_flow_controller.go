package controller

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"syncspace-backend/constants"
	"syncspace-backend/model"
	"syncspace-backend/pkg/utils"
	pkgws "syncspace-backend/pkg/ws"
	"syncspace-backend/service"
	"syncspace-backend/service/data"
)

// BoardFlowController handles HTTP requests for board-scoped flow diagrams.
type BoardFlowController struct {
	svc *service.BoardFlowService
	hub *pkgws.Hub // nil when Redis is not configured
}

// NewBoardFlowController creates a BoardFlowController backed by the given service.
// hub may be nil — diagram broadcasts are silently skipped when it is.
func NewBoardFlowController(svc *service.BoardFlowService, hub *pkgws.Hub) *BoardFlowController {
	return &BoardFlowController{svc: svc, hub: hub}
}

func boardFlowCtx(c *gin.Context) (orgID, orgRole string) {
	oid, _ := c.Get(string(constants.ContextKeyOrgID))
	orgID, _ = oid.(string)
	rol, _ := c.Get(string(constants.ContextKeyOrgRole))
	orgRole, _ = rol.(string)
	return
}

func requireOrg(c *gin.Context, orgID string) bool {
	if orgID == "" {
		c.JSON(http.StatusForbidden, data.Fail("you must belong to an organization"))
		return false
	}
	return true
}

// ── Request types ─────────────────────────────────────────────────────────────

type createFlowRequest struct {
	Name string `json:"name"`
}

type renameFlowRequest struct {
	Name string `json:"name"`
}

// ── Handlers ──────────────────────────────────────────────────────────────────

// ListFlows handles GET /api/boards/:id/flows.
func (bfc *BoardFlowController) ListFlows(c *gin.Context) {
	orgID, _ := boardFlowCtx(c)
	if !requireOrg(c, orgID) {
		return
	}
	boardID := c.Param("id")
	flows, err := bfc.svc.ListFlows(c.Request.Context(), boardID, orgID)
	if err != nil {
		utils.Error(constants.LogTagBoard, "ListFlows failed board="+boardID, err)
		c.JSON(appErrStatus(err), data.Fail(appErrMsg(err)))
		return
	}
	c.JSON(http.StatusOK, data.OK(flows))
}

// CreateFlow handles POST /api/boards/:id/flows.
func (bfc *BoardFlowController) CreateFlow(c *gin.Context) {
	orgID, orgRole := boardFlowCtx(c)
	if !requireOrg(c, orgID) {
		return
	}
	var req createFlowRequest
	_ = c.ShouldBindJSON(&req)

	boardID := c.Param("id")
	flow, err := bfc.svc.CreateFlow(c.Request.Context(), boardID, orgID, orgRole, req.Name)
	if err != nil {
		utils.Error(constants.LogTagBoard, "CreateFlow failed board="+boardID, err)
		c.JSON(appErrStatus(err), data.Fail(appErrMsg(err)))
		return
	}
	c.JSON(http.StatusCreated, data.OK(flow))
}

// GetFlow handles GET /api/boards/:id/flows/:flowId.
func (bfc *BoardFlowController) GetFlow(c *gin.Context) {
	orgID, _ := boardFlowCtx(c)
	if !requireOrg(c, orgID) {
		return
	}
	boardID := c.Param("id")
	flowID := c.Param("flowId")
	flow, err := bfc.svc.GetFlow(c.Request.Context(), boardID, orgID, flowID)
	if err != nil {
		utils.Error(constants.LogTagBoard, "GetFlow failed board="+boardID+" flow="+flowID, err)
		c.JSON(appErrStatus(err), data.Fail(appErrMsg(err)))
		return
	}
	c.JSON(http.StatusOK, data.OK(flow))
}

// RenameFlow handles PUT /api/boards/:id/flows/:flowId.
func (bfc *BoardFlowController) RenameFlow(c *gin.Context) {
	orgID, orgRole := boardFlowCtx(c)
	if !requireOrg(c, orgID) {
		return
	}
	var req renameFlowRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.Name == "" {
		c.JSON(http.StatusBadRequest, data.Fail("name is required"))
		return
	}
	boardID := c.Param("id")
	flowID := c.Param("flowId")
	flow, err := bfc.svc.RenameFlow(c.Request.Context(), boardID, orgID, orgRole, flowID, req.Name)
	if err != nil {
		utils.Error(constants.LogTagBoard, "RenameFlow failed board="+boardID+" flow="+flowID, err)
		c.JSON(appErrStatus(err), data.Fail(appErrMsg(err)))
		return
	}
	c.JSON(http.StatusOK, data.OK(flow))
}

// DeleteFlow handles DELETE /api/boards/:id/flows/:flowId.
func (bfc *BoardFlowController) DeleteFlow(c *gin.Context) {
	orgID, orgRole := boardFlowCtx(c)
	if !requireOrg(c, orgID) {
		return
	}
	boardID := c.Param("id")
	flowID := c.Param("flowId")
	if err := bfc.svc.DeleteFlow(c.Request.Context(), boardID, orgID, orgRole, flowID); err != nil {
		utils.Error(constants.LogTagBoard, "DeleteFlow failed board="+boardID+" flow="+flowID, err)
		c.JSON(appErrStatus(err), data.Fail(appErrMsg(err)))
		return
	}
	c.Status(http.StatusNoContent)
}

// DuplicateFlow handles POST /api/boards/:id/flows/:flowId/duplicate.
func (bfc *BoardFlowController) DuplicateFlow(c *gin.Context) {
	orgID, orgRole := boardFlowCtx(c)
	if !requireOrg(c, orgID) {
		return
	}
	var req struct {
		Name string `json:"name"`
	}
	_ = c.ShouldBindJSON(&req)

	boardID := c.Param("id")
	flowID := c.Param("flowId")
	flow, err := bfc.svc.DuplicateFlow(c.Request.Context(), boardID, orgID, orgRole, flowID, req.Name)
	if err != nil {
		utils.Error(constants.LogTagBoard, "DuplicateFlow failed board="+boardID+" flow="+flowID, err)
		c.JSON(appErrStatus(err), data.Fail(appErrMsg(err)))
		return
	}
	c.JSON(http.StatusCreated, data.OK(flow))
}

// GetDiagram handles GET /api/boards/:id/flows/:flowId/diagram.
func (bfc *BoardFlowController) GetDiagram(c *gin.Context) {
	orgID, _ := boardFlowCtx(c)
	if !requireOrg(c, orgID) {
		return
	}
	boardID := c.Param("id")
	flowID := c.Param("flowId")
	diagram, err := bfc.svc.GetDiagram(c.Request.Context(), boardID, orgID, flowID)
	if err != nil {
		utils.Error(constants.LogTagBoard, "GetDiagram failed board="+boardID+" flow="+flowID, err)
		c.JSON(appErrStatus(err), data.Fail(appErrMsg(err)))
		return
	}
	c.JSON(http.StatusOK, data.OK(diagram))
}

// SaveDiagram handles PUT /api/boards/:id/flows/:flowId/diagram.
func (bfc *BoardFlowController) SaveDiagram(c *gin.Context) {
	orgID, orgRole := boardFlowCtx(c)
	if !requireOrg(c, orgID) {
		return
	}
	var d model.DiagramData
	if err := c.ShouldBindJSON(&d); err != nil {
		c.JSON(http.StatusBadRequest, data.Fail("invalid diagram payload"))
		return
	}
	boardID := c.Param("id")
	flowID := c.Param("flowId")
	updatedAt, nodeCount, edgeCount, err := bfc.svc.SaveDiagram(c.Request.Context(), boardID, orgID, orgRole, flowID, &d)
	if err != nil {
		utils.Error(constants.LogTagBoard, "SaveDiagram failed board="+boardID+" flow="+flowID, err)
		c.JSON(appErrStatus(err), data.Fail(appErrMsg(err)))
		return
	}

	if bfc.hub != nil {
		senderID := mustUserID(c)
		bfc.hub.PublishToFlowExcluding(c.Request.Context(), flowID, senderID, pkgws.Message{
			Type: pkgws.TypeDiagramUpdated,
			Payload: pkgws.MustMarshal(pkgws.DiagramUpdatedPayload{
				Nodes: d.Nodes,
				Edges: d.Edges,
			}),
		})
	}

	c.JSON(http.StatusOK, data.OK(gin.H{
		"updatedAt": updatedAt,
		"nodeCount": nodeCount,
		"edgeCount": edgeCount,
	}))
}
