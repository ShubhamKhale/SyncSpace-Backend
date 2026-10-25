package controller

import (
	"encoding/json"
	"net/http"

	"github.com/gin-gonic/gin"

	"syncspace-backend/constants"
	"syncspace-backend/pkg/utils"
	"syncspace-backend/service"
	"syncspace-backend/service/data"
)

// FlowAIConversationController handles per-user, per-flow AI conversation persistence.
type FlowAIConversationController struct {
	svc *service.FlowAIConversationService
}

// NewFlowAIConversationController creates a FlowAIConversationController.
func NewFlowAIConversationController(svc *service.FlowAIConversationService) *FlowAIConversationController {
	return &FlowAIConversationController{svc: svc}
}

// GetConversation handles GET /api/flows/:flowId/ai-conversation.
// Returns {history, messages} or {history:[], messages:[]} when none exists yet.
func (fc *FlowAIConversationController) GetConversation(c *gin.Context) {
	userID := mustUserID(c)
	flowID := c.Param("flowId")

	conv, err := fc.svc.Get(c.Request.Context(), userID, flowID)
	if err != nil {
		utils.Error(constants.LogTagAI, "GetConversation failed for user="+userID+" flow="+flowID, err)
		c.JSON(appErrStatus(err), data.Fail(appErrMsg(err)))
		return
	}

	if conv == nil {
		c.JSON(http.StatusOK, data.OK(gin.H{
			"history":  json.RawMessage("[]"),
			"messages": json.RawMessage("[]"),
		}))
		return
	}

	c.JSON(http.StatusOK, data.OK(gin.H{
		"history":  conv.History,
		"messages": conv.Messages,
	}))
}

type saveConversationRequest struct {
	History  json.RawMessage `json:"history"  binding:"required"`
	Messages json.RawMessage `json:"messages" binding:"required"`
}

// SaveConversation handles PUT /api/flows/:flowId/ai-conversation.
// Upserts the conversation state for the authenticated user + flow.
func (fc *FlowAIConversationController) SaveConversation(c *gin.Context) {
	userID := mustUserID(c)
	flowID := c.Param("flowId")

	var req saveConversationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, data.Fail(err.Error()))
		return
	}

	if err := fc.svc.Save(c.Request.Context(), userID, flowID, req.History, req.Messages); err != nil {
		utils.Error(constants.LogTagAI, "SaveConversation failed for user="+userID+" flow="+flowID, err)
		c.JSON(appErrStatus(err), data.Fail(appErrMsg(err)))
		return
	}

	c.JSON(http.StatusOK, data.OK(nil))
}
