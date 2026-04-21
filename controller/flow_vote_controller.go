package controller

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"syncspace-backend/constants"
	"syncspace-backend/pkg/utils"
	"syncspace-backend/service"
	"syncspace-backend/service/data"
)

// FlowVoteController handles vote endpoints for flow diagrams.
type FlowVoteController struct {
	svc *service.FlowVoteService
}

// NewFlowVoteController creates a FlowVoteController backed by the given service.
func NewFlowVoteController(svc *service.FlowVoteService) *FlowVoteController {
	return &FlowVoteController{svc: svc}
}

// ── Request types ─────────────────────────────────────────────────────────────

// castVoteRequest is the POST /api/flows/:id/votes body.
type castVoteRequest struct {
	VoteType string `json:"vote_type" binding:"required"`
}

// ── Handlers ──────────────────────────────────────────────────────────────────

// CastVote handles POST /api/flows/:id/votes.
// Creates a new vote or updates the caller's existing vote (one per user per flow).
//
// Request body:
//
//	{ "vote_type": "up" }   or   { "vote_type": "down" }
//
// Response: the persisted vote record.
func (vc *FlowVoteController) CastVote(c *gin.Context) {
	callerID := mustUserID(c)
	flowID   := c.Param("id")

	var req castVoteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, data.Fail(err.Error()))
		return
	}

	vote, err := vc.svc.CastVote(c.Request.Context(), flowID, callerID, req.VoteType)
	if err != nil {
		utils.Error(constants.LogTagFlow, "CastVote failed flow="+flowID, err)
		c.JSON(appErrStatus(err), data.Fail(appErrMsg(err)))
		return
	}

	utils.Info(constants.LogTagFlow, "vote cast: flow="+flowID+" user="+callerID+" type="+req.VoteType)
	c.JSON(http.StatusOK, data.OK(vote))
}

// GetVotes handles GET /api/flows/:id/votes.
// Returns vote totals and the caller's current vote for the flow.
//
// Response:
//
//	{ "up": 5, "down": 1, "my_vote": "up" }
//
// my_vote is "" when the caller has not voted.
func (vc *FlowVoteController) GetVotes(c *gin.Context) {
	callerID := mustUserID(c)
	flowID   := c.Param("id")

	summary, err := vc.svc.GetSummary(c.Request.Context(), flowID, callerID)
	if err != nil {
		utils.Error(constants.LogTagFlow, "GetVotes failed flow="+flowID, err)
		c.JSON(appErrStatus(err), data.Fail(appErrMsg(err)))
		return
	}

	c.JSON(http.StatusOK, data.OK(summary))
}
