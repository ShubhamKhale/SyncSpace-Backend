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

type createOrgRequest struct {
	Name string `json:"name" binding:"required,max=255"`
}

type updateOrgRequest struct {
	Name        *string `json:"name"`
	Description *string `json:"description"`
}

type inviteMemberRequest struct {
	OrgID string `json:"org_id" binding:"required"`
	Email string `json:"email"  binding:"required,email"`
	Role  string `json:"role"   binding:"required"`
}

type sendInviteRequest struct {
	Email string `json:"email" binding:"required,email"`
	Role  string `json:"role"  binding:"required"`
}

type updateMemberRoleRequest struct {
	Role string `json:"role" binding:"required"`
}

// ── Helpers ───────────────────────────────────────────────────────────────────

// mustOrgID reads the org ID from the OrgContext middleware.
// Returns empty string if user has no org (caller must handle).
func mustOrgID(c *gin.Context) string {
	val, _ := c.Get(string(constants.ContextKeyOrgID))
	if val == nil {
		return ""
	}
	return val.(string)
}

// ── Organization handlers ─────────────────────────────────────────────────────

// CreateOrganization handles POST /api/organization.
func (oc *OrgController) CreateOrganization(c *gin.Context) {
	callerID := mustUserID(c)

	var req createOrgRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, data.Fail(err.Error()))
		return
	}

	org, err := oc.svc.CreateOrg(c.Request.Context(), callerID, req.Name)
	if err != nil {
		utils.Error(constants.LogTagOrg, "CreateOrganization failed for "+callerID, err)
		c.JSON(appErrStatus(err), data.Fail(appErrMsg(err)))
		return
	}

	utils.Info(constants.LogTagOrg, "organization created: "+org.ID+" by "+callerID)
	c.JSON(http.StatusCreated, data.OK(gin.H{"id": org.ID, "name": org.Name}))
}

// GetOrganization handles GET /api/organization.
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
		utils.Error(constants.LogTagOrg, "UpdateOrganization failed org="+orgID, err)
		c.JSON(appErrStatus(err), data.Fail(appErrMsg(err)))
		return
	}

	utils.Info(constants.LogTagOrg, "organization updated: "+orgID+" by "+callerID)
	c.JSON(http.StatusOK, data.OK(org))
}

// ── Member handlers ───────────────────────────────────────────────────────────

// GetMembers handles GET /api/organization/members.
// orgID is read from OrgContext middleware — no query param needed.
func (oc *OrgController) GetMembers(c *gin.Context) {
	callerID := mustUserID(c)
	orgID := mustOrgID(c)
	if orgID == "" {
		c.JSON(http.StatusForbidden, data.Fail("you must belong to an organization"))
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

// InviteMember handles POST /api/organization/members/invite (direct membership invite).
func (oc *OrgController) InviteMember(c *gin.Context) {
	callerID := mustUserID(c)

	var req inviteMemberRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, data.Fail(err.Error()))
		return
	}

	detail, err := oc.svc.InviteMember(c.Request.Context(), callerID, req.OrgID, req.Email, req.Role)
	if err != nil {
		utils.Error(constants.LogTagOrg, "InviteMember failed org="+req.OrgID, err)
		c.JSON(appErrStatus(err), data.Fail(appErrMsg(err)))
		return
	}

	utils.Info(constants.LogTagOrg, "member invited: "+req.Email+" to org "+req.OrgID)
	c.JSON(http.StatusCreated, data.OK(detail))
}

// SendInvite handles POST /api/organization/invite (token-link invite flow).
func (oc *OrgController) SendInvite(c *gin.Context) {
	callerID := mustUserID(c)

	var req sendInviteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, data.Fail(err.Error()))
		return
	}

	invite, err := oc.svc.SendInvite(c.Request.Context(), callerID, req.Email, req.Role)
	if err != nil {
		utils.Error(constants.LogTagOrg, "SendInvite failed caller="+callerID, err)
		c.JSON(appErrStatus(err), data.Fail(appErrMsg(err)))
		return
	}

	utils.Info(constants.LogTagOrg, "invite token created for "+req.Email+" by "+callerID)
	c.JSON(http.StatusCreated, data.OK(gin.H{"inviteId": invite.ID, "token": invite.Token}))
}

// VerifyInvite handles GET /api/organization/invite/verify?token=<uuid>.
// Public endpoint — no JWT required.
func (oc *OrgController) VerifyInvite(c *gin.Context) {
	token := c.Query("token")
	if token == "" {
		c.JSON(http.StatusBadRequest, data.Fail("token query parameter is required"))
		return
	}

	result, err := oc.svc.VerifyInvite(c.Request.Context(), token)
	if err != nil {
		utils.Error(constants.LogTagOrg, "VerifyInvite failed", err)
		c.JSON(appErrStatus(err), data.Fail(appErrMsg(err)))
		return
	}

	c.JSON(http.StatusOK, data.OK(result))
}

// UpdateMemberRole handles PATCH /api/organization/members/:id/role.
// orgID read from OrgContext middleware.
func (oc *OrgController) UpdateMemberRole(c *gin.Context) {
	callerID := mustUserID(c)
	targetUserID := c.Param("id")
	orgID := mustOrgID(c)
	if orgID == "" {
		c.JSON(http.StatusForbidden, data.Fail("you must belong to an organization"))
		return
	}

	var req updateMemberRoleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, data.Fail(err.Error()))
		return
	}

	if err := oc.svc.UpdateMemberRole(c.Request.Context(), callerID, orgID, targetUserID, req.Role); err != nil {
		utils.Error(constants.LogTagOrg, "UpdateMemberRole failed user="+targetUserID, err)
		c.JSON(appErrStatus(err), data.Fail(appErrMsg(err)))
		return
	}

	utils.Info(constants.LogTagOrg, "role updated: user="+targetUserID+" role="+req.Role)
	c.JSON(http.StatusOK, data.OK(gin.H{"user_id": targetUserID, "role": req.Role}))
}

// RemoveMember handles DELETE /api/organization/members/:id.
// orgID read from OrgContext middleware.
func (oc *OrgController) RemoveMember(c *gin.Context) {
	callerID := mustUserID(c)
	targetUserID := c.Param("id")
	orgID := mustOrgID(c)
	if orgID == "" {
		c.JSON(http.StatusForbidden, data.Fail("you must belong to an organization"))
		return
	}

	if err := oc.svc.RemoveMember(c.Request.Context(), callerID, orgID, targetUserID); err != nil {
		utils.Error(constants.LogTagOrg, "RemoveMember failed user="+targetUserID, err)
		c.JSON(appErrStatus(err), data.Fail(appErrMsg(err)))
		return
	}

	utils.Info(constants.LogTagOrg, "member removed: user="+targetUserID+" from org "+orgID)
	c.JSON(http.StatusOK, data.OK(gin.H{"removed": true}))
}
