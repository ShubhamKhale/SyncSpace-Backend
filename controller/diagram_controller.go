package controller

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"syncspace-backend/constants"
	"syncspace-backend/errs"
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
	// WebSearch opts in to grounding via Groq's browser_search (≈3x tokens).
	WebSearch bool `json:"web_search"`
}

type conversationTurnRequest struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type generateDiagramConversationalRequest struct {
	History   []conversationTurnRequest `json:"history" binding:"required,min=1"`
	WebSearch bool                      `json:"web_search"`
}

// GenerateDiagram handles POST /api/ai/generate-diagram.
// Returns a {title, nodes, edges} graph generated from the prompt. When every
// Groq model is rate-limited it responds 429 with retry_after (seconds) so the
// client can count down and retry automatically.
func (dc *DiagramController) GenerateDiagram(c *gin.Context) {
	var req generateDiagramRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, data.Fail(err.Error()))
		return
	}

	graph, err := dc.svc.GenerateDiagram(c.Request.Context(), req.Prompt, service.DiagramOptions{WebSearch: req.WebSearch})
	if err != nil {
		utils.Error(constants.LogTagAI, "GenerateDiagram failed", err)
		resp := data.Fail(appErrMsg(err))
		var appErr *errs.AppError
		if errors.As(err, &appErr) {
			resp.RetryAfter = appErr.RetryAfter
		}
		c.JSON(appErrStatus(err), resp)
		return
	}

	c.JSON(http.StatusOK, data.OK(graph))
}

// GenerateDiagramConversational handles POST /api/ai/generate-diagram/chat.
// Client sends full conversation history each turn; response is either
// {"action":"ask","question":"..."} or {"action":"diagram","graph":{...}}.
func (dc *DiagramController) GenerateDiagramConversational(c *gin.Context) {
	var req generateDiagramConversationalRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, data.Fail(err.Error()))
		return
	}

	history := make([]service.ConversationTurn, len(req.History))
	for i, t := range req.History {
		history[i] = service.ConversationTurn{Role: t.Role, Content: t.Content}
	}

	resp, err := dc.svc.GenerateDiagramConversational(
		c.Request.Context(),
		history,
		service.DiagramOptions{WebSearch: req.WebSearch},
	)
	if err != nil {
		utils.Error(constants.LogTagAI, "GenerateDiagramConversational failed", err)
		apiResp := data.Fail(appErrMsg(err))
		var appErr *errs.AppError
		if errors.As(err, &appErr) {
			apiResp.RetryAfter = appErr.RetryAfter
		}
		c.JSON(appErrStatus(err), apiResp)
		return
	}

	c.JSON(http.StatusOK, data.OK(resp))
}
