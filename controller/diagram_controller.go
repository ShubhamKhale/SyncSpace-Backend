package controller

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"syncspace-backend/constants"
	"syncspace-backend/pkg/utils"
	"syncspace-backend/service"
	"syncspace-backend/service/data"
)

// DiagramController handles AI architecture-diagram generation.
type DiagramController struct {
	svc *service.DiagramService
}

// NewDiagramController creates a DiagramController backed by the given service.
func NewDiagramController(svc *service.DiagramService) *DiagramController {
	return &DiagramController{svc: svc}
}

type generateDiagramRequest struct {
	Prompt string `json:"prompt" binding:"required"`
}

// GenerateDiagram handles POST /api/ai/generate-diagram.
// Returns a {title, nodes, edges} graph generated from the prompt via
// Groq's web-search-backed compound model.
func (dc *DiagramController) GenerateDiagram(c *gin.Context) {
	var req generateDiagramRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, data.Fail(err.Error()))
		return
	}

	graph, err := dc.svc.GenerateDiagram(c.Request.Context(), req.Prompt)
	if err != nil {
		utils.Error(constants.LogTagAI, "GenerateDiagram failed", err)
		c.JSON(appErrStatus(err), data.Fail(appErrMsg(err)))
		return
	}

	c.JSON(http.StatusOK, data.OK(graph))
}
