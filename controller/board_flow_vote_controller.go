package controller

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"syncspace-backend/constants"
	"syncspace-backend/pkg/utils"
	"syncspace-backend/service"
	"syncspace-backend/service/data"
)

// BoardFlowVoteController handles vote endpoints for board flow diagrams.
type BoardFlowVoteController struct {
	svc *service.BoardFlowVoteService
}

// NewBoardFlowVoteController creates a BoardFlowVoteController.
func NewBoardFlowVoteController(svc *service.BoardFlowVoteService) *BoardFlowVoteController {
	return &BoardFlowVoteController{svc: svc}
}

type castBoardFlowVoteRequest struct {
	Vote string `json:"vote" binding:"required"`
}

// GetVotes handles GET /api/boards/:id/flows/:flowId/votes.
func (vc *BoardFlowVoteController) GetVotes(c *gin.Context) {
	orgID, _ := boardFlowCtx(c)
	if !requireOrg(c, orgID) {
		return
	}
	boardID := c.Param("id")
	flowID := c.Param("flowId")

	votes, err := vc.svc.GetVotes(c.Request.Context(), boardID, orgID, flowID)
	if err != nil {
		utils.Error(constants.LogTagBoard, "GetVotes failed board="+boardID+" flow="+flowID, err)
		c.JSON(appErrStatus(err), data.Fail(appErrMsg(err)))
		return
	}
	c.JSON(http.StatusOK, data.OK(votes))
}

// CastVote handles POST /api/boards/:id/flows/:flowId/votes.
func (vc *BoardFlowVoteController) CastVote(c *gin.Context) {
	orgID, _ := boardFlowCtx(c)
	if !requireOrg(c, orgID) {
		return
	}
	var req castBoardFlowVoteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, data.Fail("vote is required"))
		return
	}
	boardID := c.Param("id")
	flowID := c.Param("flowId")
	userID := mustUserID(c)

	vote, err := vc.svc.CastVote(c.Request.Context(), boardID, orgID, flowID, userID, req.Vote)
	if err != nil {
		utils.Error(constants.LogTagBoard, "CastVote failed board="+boardID+" flow="+flowID, err)
		c.JSON(appErrStatus(err), data.Fail(appErrMsg(err)))
		return
	}
	c.JSON(http.StatusOK, data.OK(vote))
}

// RemoveVote handles DELETE /api/boards/:id/flows/:flowId/votes.
func (vc *BoardFlowVoteController) RemoveVote(c *gin.Context) {
	orgID, _ := boardFlowCtx(c)
	if !requireOrg(c, orgID) {
		return
	}
	boardID := c.Param("id")
	flowID := c.Param("flowId")
	userID := mustUserID(c)

	if err := vc.svc.RemoveVote(c.Request.Context(), boardID, orgID, flowID, userID); err != nil {
		utils.Error(constants.LogTagBoard, "RemoveVote failed board="+boardID+" flow="+flowID, err)
		c.JSON(appErrStatus(err), data.Fail(appErrMsg(err)))
		return
	}
	c.Status(http.StatusNoContent)
}
