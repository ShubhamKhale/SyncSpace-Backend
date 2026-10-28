package controller

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"syncspace-backend/constants"
	"syncspace-backend/pkg/utils"
	"syncspace-backend/service"
	"syncspace-backend/service/data"
)

type AttendanceController struct {
	svc *service.AttendanceService
}

func NewAttendanceController(svc *service.AttendanceService) *AttendanceController {
	return &AttendanceController{svc: svc}
}

type checkInRequest struct {
	Note string `json:"note"`
}

func (ac *AttendanceController) CheckIn(c *gin.Context) {
	orgID, ok := attendanceAccess(c)
	if !ok {
		return
	}
	userID := mustUserID(c)

	var req checkInRequest
	_ = c.ShouldBindJSON(&req)

	log, err := ac.svc.CheckIn(c.Request.Context(), userID, orgID, req.Note)
	if err != nil {
		utils.Error(constants.LogTagAttendance, "check-in failed", err)
		c.JSON(appErrStatus(err), data.Fail(appErrMsg(err)))
		return
	}
	c.JSON(http.StatusOK, data.OK(log))
}

func (ac *AttendanceController) CheckOut(c *gin.Context) {
	orgID, ok := attendanceAccess(c)
	if !ok {
		return
	}
	userID := mustUserID(c)

	log, err := ac.svc.CheckOut(c.Request.Context(), userID, orgID)
	if err != nil {
		utils.Error(constants.LogTagAttendance, "check-out failed", err)
		c.JSON(appErrStatus(err), data.Fail(appErrMsg(err)))
		return
	}
	c.JSON(http.StatusOK, data.OK(log))
}

func (ac *AttendanceController) GetMyStatus(c *gin.Context) {
	orgID, ok := attendanceAccess(c)
	if !ok {
		return
	}
	userID := mustUserID(c)

	log, err := ac.svc.GetMyStatus(c.Request.Context(), userID, orgID)
	if err != nil {
		utils.Error(constants.LogTagAttendance, "get status failed", err)
		c.JSON(appErrStatus(err), data.Fail(appErrMsg(err)))
		return
	}
	if log == nil {
		c.JSON(http.StatusOK, data.OK(gin.H{"status": "absent"}))
		return
	}
	status := "checked_out"
	if log.CheckOutAt == nil {
		status = "checked_in"
	}
	c.JSON(http.StatusOK, data.OK(gin.H{
		"status": status,
		"log":    log,
	}))
}

func (ac *AttendanceController) GetTeamStatus(c *gin.Context) {
	orgID, ok := adminAccess(c)
	if !ok {
		return
	}

	summary, err := ac.svc.GetTeamStatus(c.Request.Context(), orgID)
	if err != nil {
		utils.Error(constants.LogTagAttendance, "get team status failed", err)
		c.JSON(appErrStatus(err), data.Fail(appErrMsg(err)))
		return
	}
	c.JSON(http.StatusOK, data.OK(summary))
}

func (ac *AttendanceController) GetMemberAvailability(c *gin.Context) {
	orgID, ok := attendanceAccess(c)
	if !ok {
		return
	}

	members, err := ac.svc.GetMemberAvailability(c.Request.Context(), orgID)
	if err != nil {
		utils.Error(constants.LogTagAttendance, "get availability failed", err)
		c.JSON(appErrStatus(err), data.Fail(appErrMsg(err)))
		return
	}
	c.JSON(http.StatusOK, data.OK(members))
}

func attendanceAccess(c *gin.Context) (orgID string, ok bool) {
	oid, _ := c.Get(string(constants.ContextKeyOrgID))
	orgID, _ = oid.(string)
	if orgID == "" {
		c.JSON(http.StatusForbidden, data.Fail("you must belong to an organization"))
		return "", false
	}
	return orgID, true
}

var adminRoles = map[string]bool{"owner": true, "admin": true}

func adminAccess(c *gin.Context) (orgID string, ok bool) {
	orgID, ok = attendanceAccess(c)
	if !ok {
		return
	}
	r, _ := c.Get(string(constants.ContextKeyOrgRole))
	role, _ := r.(string)
	if !adminRoles[role] {
		c.JSON(http.StatusForbidden, data.Fail("only admins and owners can access this"))
		return "", false
	}
	return orgID, true
}
