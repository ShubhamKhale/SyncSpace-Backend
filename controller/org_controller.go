package controller

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"syncspace-backend/constants"
	"syncspace-backend/pkg/utils"
	"syncspace-backend/service"
	"syncspace-backend/service/data"
)

// OrgController handles organization endpoints.
type OrgController struct {
	svc *service.OrgService
}

// NewOrgController creates an OrgController backed by the given OrgService.
func NewOrgController(svc *service.OrgService) *OrgController {
	return &OrgController{svc: svc}
}

// ── Request types ─────────────────────────────────────────────────────────────

// updateOrgRequest uses pointer fields for true partial updates.
// At least one field must be non-nil or the handler returns 400.
type updateOrgRequest struct {
	Name        *string `json:"name"`
	Description *string `json:"description"`
}

type inviteMemberRequest struct {
	OrgID string `json:"org_id" binding:"required"`
	Email string `json:"email"  binding:"required,email"`
	Role  string `json:"role"   binding:"required"`
}

type updateMemberRoleRequest struct {
	Role string `json:"role" binding:"required"`
}

// ── Handlers ──────────────────────────────────────────────────────────────────

// GetOrganization handles GET /api/organization.
// Returns every organization the authenticated user belongs to, together with
// their role in each (owner | admin | member | viewer).
func (oc *OrgController) GetOrganization(c *gin.Context) {
	callerID := mustUserID(c)

	orgs, err := oc.svc.GetMyOrgs(c.Request.Context(), callerID)
	if err != nil {
		utils.Error(constants.LogTagOrg, "GetOrganization failed for "+callerID, err)
		c.JSON(appErrStatus(err), data.Fail(appErrMsg(err)))
		return
	}

	c.JSON(http.StatusOK, data.OK(orgs))
}

// UpdateOrganization handles PATCH /api/organization/:id.
// Updates the name and/or description of the specified organization.
// The caller must hold the "owner" or "admin" role — the service enforces this.
func (oc *OrgController) UpdateOrganization(c *gin.Context) {
	callerID := mustUserID(c)
	orgID := c.Param("id")
	if orgID == "" {
		c.JSON(http.StatusBadRequest, data.Fail("organization id is required"))
		return
	}

	var req updateOrgRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, data.Fail(err.Error()))
		return
	}

	if req.Name == nil && req.Description == nil {
		c.JSON(http.StatusBadRequest, data.Fail("provide at least one field to update"))
		return
	}

	org, err := oc.svc.UpdateOrg(c.Request.Context(), callerID, orgID, req.Name, req.Description)
	if err != nil {
		utils.Error(constants.LogTagOrg, "UpdateOrganization failed org="+orgID+" caller="+callerID, err)
		c.JSON(appErrStatus(err), data.Fail(appErrMsg(err)))
		return
	}

	utils.Info(constants.LogTagOrg, "organization updated: "+orgID+" by "+callerID)
	c.JSON(http.StatusOK, data.OK(org))
}

// ── Member handlers ───────────────────────────────────────────────────────────

// GetMembers handles GET /api/organization/members?org_id=xxx.
// Any member of the organization can list members (with their status and role).
func (oc *OrgController) GetMembers(c *gin.Context) {
	callerID := mustUserID(c)
	orgID := c.Query("org_id")
	if orgID == "" {
		c.JSON(http.StatusBadRequest, data.Fail("org_id query parameter is required"))
		return
	}

	members, err := oc.svc.GetMembers(c.Request.Context(), callerID, orgID)
	if err != nil {
		utils.Error(constants.LogTagOrg, "GetMembers failed org="+orgID, err)
		c.JSON(appErrStatus(err), data.Fail(appErrMsg(err)))
		return
	}

	c.JSON(http.StatusOK, data.OK(members))
}

// InviteMember handles POST /api/organization/members/invite.
// Looks up the target user by email and creates a membership with status "invited".
// Only owner / admin may invite.
func (oc *OrgController) InviteMember(c *gin.Context) {
	callerID := mustUserID(c)

	var req inviteMemberRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, data.Fail(err.Error()))
		return
	}

	detail, err := oc.svc.InviteMember(c.Request.Context(), callerID, req.OrgID, req.Email, req.Role)
	if err != nil {
		utils.Error(constants.LogTagOrg, "InviteMember failed org="+req.OrgID+" email="+req.Email, err)
		c.JSON(appErrStatus(err), data.Fail(appErrMsg(err)))
		return
	}

	utils.Info(constants.LogTagOrg, "member invited: "+req.Email+" to org "+req.OrgID+" by "+callerID)
	c.JSON(http.StatusCreated, data.OK(detail))
}

// UpdateMemberRole handles PATCH /api/organization/members/:id/role?org_id=xxx.
// :id is the target member's user ID. Only owner / admin may change roles.
func (oc *OrgController) UpdateMemberRole(c *gin.Context) {
	callerID    := mustUserID(c)
	targetUserID := c.Param("id")
	orgID        := c.Query("org_id")

	if orgID == "" {
		c.JSON(http.StatusBadRequest, data.Fail("org_id query parameter is required"))
		return
	}

	var req updateMemberRoleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, data.Fail(err.Error()))
		return
	}

	if err := oc.svc.UpdateMemberRole(c.Request.Context(), callerID, orgID, targetUserID, req.Role); err != nil {
		utils.Error(constants.LogTagOrg, "UpdateMemberRole failed user="+targetUserID+" org="+orgID, err)
		c.JSON(appErrStatus(err), data.Fail(appErrMsg(err)))
		return
	}

	utils.Info(constants.LogTagOrg, "role updated: user="+targetUserID+" role="+req.Role+" org="+orgID+" by "+callerID)
	c.JSON(http.StatusOK, data.OK(gin.H{"user_id": targetUserID, "role": req.Role}))
}

// RemoveMember handles DELETE /api/organization/members/:id?org_id=xxx.
// :id is the target member's user ID.
// Any member may remove themselves (self-leave). Removing others requires owner / admin.
func (oc *OrgController) RemoveMember(c *gin.Context) {
	callerID     := mustUserID(c)
	targetUserID := c.Param("id")
	orgID         := c.Query("org_id")

	if orgID == "" {
		c.JSON(http.StatusBadRequest, data.Fail("org_id query parameter is required"))
		return
	}

	if err := oc.svc.RemoveMember(c.Request.Context(), callerID, orgID, targetUserID); err != nil {
		utils.Error(constants.LogTagOrg, "RemoveMember failed user="+targetUserID+" org="+orgID, err)
		c.JSON(appErrStatus(err), data.Fail(appErrMsg(err)))
		return
	}

	utils.Info(constants.LogTagOrg, "member removed: user="+targetUserID+" from org "+orgID+" by "+callerID)
	c.JSON(http.StatusOK, data.OK(gin.H{"removed": true}))
}
