package controller

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"syncspace-backend/constants"
	"syncspace-backend/pkg/utils"
	"syncspace-backend/service"
	"syncspace-backend/service/data"
)

type LeaveController struct {
	svc *service.LeaveService
}

func NewLeaveController(svc *service.LeaveService) *LeaveController {
	return &LeaveController{svc: svc}
}

func (lc *LeaveController) GetLeaveTypes(c *gin.Context) {
	orgID, ok := attendanceAccess(c)
	if !ok {
		return
	}
	types, err := lc.svc.GetLeaveTypes(c.Request.Context(), orgID)
	if err != nil {
		utils.Error(constants.LogTagLeave, "get leave types failed", err)
		c.JSON(appErrStatus(err), data.Fail(appErrMsg(err)))
		return
	}
	c.JSON(http.StatusOK, data.OK(types))
}

type createLeaveTypeRequest struct {
	Name        string `json:"name"         binding:"required"`
	DefaultDays int    `json:"default_days" binding:"required,min=1"`
}

func (lc *LeaveController) CreateLeaveType(c *gin.Context) {
	orgID, ok := adminAccess(c)
	if !ok {
		return
	}
	var req createLeaveTypeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, data.Fail(err.Error()))
		return
	}
	lt, err := lc.svc.CreateLeaveType(c.Request.Context(), orgID, req.Name, req.DefaultDays)
	if err != nil {
		utils.Error(constants.LogTagLeave, "create leave type failed", err)
		c.JSON(appErrStatus(err), data.Fail(appErrMsg(err)))
		return
	}
	c.JSON(http.StatusCreated, data.OK(lt))
}

type updateLeaveTypeRequest struct {
	Name        string `json:"name"         binding:"required"`
	DefaultDays int    `json:"default_days" binding:"required,min=1"`
}

func (lc *LeaveController) UpdateLeaveType(c *gin.Context) {
	_, ok := adminAccess(c)
	if !ok {
		return
	}
	id := c.Param("id")
	var req updateLeaveTypeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, data.Fail(err.Error()))
		return
	}
	if err := lc.svc.UpdateLeaveType(c.Request.Context(), id, req.Name, req.DefaultDays); err != nil {
		utils.Error(constants.LogTagLeave, "update leave type failed", err)
		c.JSON(appErrStatus(err), data.Fail(appErrMsg(err)))
		return
	}
	c.JSON(http.StatusOK, data.OK(nil))
}

func (lc *LeaveController) GetMyBalances(c *gin.Context) {
	orgID, ok := attendanceAccess(c)
	if !ok {
		return
	}
	userID := mustUserID(c)
	balances, err := lc.svc.GetMyBalances(c.Request.Context(), userID, orgID)
	if err != nil {
		utils.Error(constants.LogTagLeave, "get balances failed", err)
		c.JSON(appErrStatus(err), data.Fail(appErrMsg(err)))
		return
	}
	c.JSON(http.StatusOK, data.OK(balances))
}

type adjustBalanceRequest struct {
	LeaveTypeID string `json:"leave_type_id" binding:"required"`
	Allocated   int    `json:"allocated"     binding:"required,min=0"`
}

func (lc *LeaveController) AdjustBalance(c *gin.Context) {
	_, ok := adminAccess(c)
	if !ok {
		return
	}
	targetUserID := c.Param("userId")
	var req adjustBalanceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, data.Fail(err.Error()))
		return
	}
	if err := lc.svc.AdjustBalance(c.Request.Context(), targetUserID, req.LeaveTypeID, req.Allocated); err != nil {
		utils.Error(constants.LogTagLeave, "adjust balance failed", err)
		c.JSON(appErrStatus(err), data.Fail(appErrMsg(err)))
		return
	}
	c.JSON(http.StatusOK, data.OK(nil))
}

type requestLeaveRequest struct {
	LeaveTypeID string `json:"leave_type_id" binding:"required"`
	StartDate   string `json:"start_date"    binding:"required"`
	EndDate     string `json:"end_date"      binding:"required"`
	Reason      string `json:"reason"`
}

func (lc *LeaveController) RequestLeave(c *gin.Context) {
	orgID, ok := attendanceAccess(c)
	if !ok {
		return
	}
	userID := mustUserID(c)
	var req requestLeaveRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, data.Fail(err.Error()))
		return
	}
	lr, err := lc.svc.RequestLeave(c.Request.Context(), userID, orgID, req.LeaveTypeID, req.StartDate, req.EndDate, req.Reason)
	if err != nil {
		utils.Error(constants.LogTagLeave, "request leave failed", err)
		c.JSON(appErrStatus(err), data.Fail(appErrMsg(err)))
		return
	}
	c.JSON(http.StatusCreated, data.OK(lr))
}

func (lc *LeaveController) GetMyRequests(c *gin.Context) {
	orgID, ok := attendanceAccess(c)
	if !ok {
		return
	}
	userID := mustUserID(c)
	reqs, err := lc.svc.GetMyRequests(c.Request.Context(), userID, orgID)
	if err != nil {
		utils.Error(constants.LogTagLeave, "get my requests failed", err)
		c.JSON(appErrStatus(err), data.Fail(appErrMsg(err)))
		return
	}
	c.JSON(http.StatusOK, data.OK(reqs))
}

func (lc *LeaveController) GetTeamRequests(c *gin.Context) {
	orgID, ok := adminAccess(c)
	if !ok {
		return
	}
	status := c.Query("status")
	reqs, err := lc.svc.GetTeamRequests(c.Request.Context(), orgID, status)
	if err != nil {
		utils.Error(constants.LogTagLeave, "get team requests failed", err)
		c.JSON(appErrStatus(err), data.Fail(appErrMsg(err)))
		return
	}
	c.JSON(http.StatusOK, data.OK(reqs))
}

func (lc *LeaveController) ApproveLeave(c *gin.Context) {
	_, ok := adminAccess(c)
	if !ok {
		return
	}
	reviewerID := mustUserID(c)
	requestID := c.Param("id")
	if err := lc.svc.ApproveLeave(c.Request.Context(), requestID, reviewerID); err != nil {
		utils.Error(constants.LogTagLeave, "approve leave failed", err)
		c.JSON(appErrStatus(err), data.Fail(appErrMsg(err)))
		return
	}
	c.JSON(http.StatusOK, data.OK(gin.H{"status": "approved"}))
}

func (lc *LeaveController) DenyLeave(c *gin.Context) {
	_, ok := adminAccess(c)
	if !ok {
		return
	}
	reviewerID := mustUserID(c)
	requestID := c.Param("id")
	if err := lc.svc.DenyLeave(c.Request.Context(), requestID, reviewerID); err != nil {
		utils.Error(constants.LogTagLeave, "deny leave failed", err)
		c.JSON(appErrStatus(err), data.Fail(appErrMsg(err)))
		return
	}
	c.JSON(http.StatusOK, data.OK(gin.H{"status": "denied"}))
}
