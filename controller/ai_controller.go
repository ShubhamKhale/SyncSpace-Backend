package controller

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"syncspace-backend/constants"
	"syncspace-backend/pkg/utils"
	"syncspace-backend/service"
	"syncspace-backend/service/data"
)

// AIController handles local-model chat and summarization endpoints.
type AIController struct {
	svc *service.AIService
}

// NewAIController creates an AIController backed by the given service.
func NewAIController(svc *service.AIService) *AIController {
	return &AIController{svc: svc}
}

// ── Request types ─────────────────────────────────────────────────────────────

type boardChatRequest struct {
	Question string `json:"question" binding:"required"`
}

type summarizeRequest struct {
	Text string `json:"text" binding:"required"`
}

// ── Handlers ──────────────────────────────────────────────────────────────────

// ChatWithBoard handles POST /api/boards/:id/ai/chat.
// Answers a question grounded in the board's tasks, description, and linked resources.
func (ac *AIController) ChatWithBoard(c *gin.Context) {
	boardID := c.Param("id")

	var req boardChatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, data.Fail(err.Error()))
		return
	}

	answer, err := ac.svc.ChatAboutBoard(c.Request.Context(), boardID, req.Question)
	if err != nil {
		utils.Error(constants.LogTagAI, "ChatWithBoard failed board="+boardID, err)
		c.JSON(appErrStatus(err), data.Fail(appErrMsg(err)))
		return
	}

	c.JSON(http.StatusOK, data.OK(gin.H{"answer": answer}))
}

// Summarize handles POST /api/ai/summarize.
// Returns a concise summary of the given text.
func (ac *AIController) Summarize(c *gin.Context) {
	var req summarizeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, data.Fail(err.Error()))
		return
	}

	summary, err := ac.svc.Summarize(c.Request.Context(), req.Text)
	if err != nil {
		utils.Error(constants.LogTagAI, "Summarize failed", err)
		c.JSON(appErrStatus(err), data.Fail(appErrMsg(err)))
		return
	}

	c.JSON(http.StatusOK, data.OK(gin.H{"summary": summary}))
}
