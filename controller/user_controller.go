package controller

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"syncspace-backend/constants"
	"syncspace-backend/pkg/utils"
	"syncspace-backend/service"
	"syncspace-backend/service/data"
)

// UserController handles user profile endpoints.
type UserController struct {
	svc *service.UserService
}

// NewUserController creates a UserController backed by the given UserService.
func NewUserController(svc *service.UserService) *UserController {
	return &UserController{svc: svc}
}

// ── Request types ─────────────────────────────────────────────────────────────

// updateProfileRequest uses pointer fields so PATCH can distinguish between
// "field not provided" (nil) and "field set to empty string" (non-nil, "").
type updateProfileRequest struct {
	Name  *string `json:"name"`
	Email *string `json:"email"  binding:"omitempty,email"`
	Bio   *string `json:"bio"`
}

type updateAvatarRequest struct {
	AvatarURL string `json:"avatar_url" binding:"required,url"`
}

// ── Handlers ──────────────────────────────────────────────────────────────────

// GetMe handles GET /api/user/me.
// Returns the full profile of the authenticated user.
func (uc *UserController) GetMe(c *gin.Context) {
	userID := mustUserID(c)

	user, err := uc.svc.GetProfile(c.Request.Context(), userID)
	if err != nil {
		utils.Error(constants.LogTagUser, "GetMe failed for "+userID, err)
		c.JSON(appErrStatus(err), data.Fail(appErrMsg(err)))
		return
	}

	c.JSON(http.StatusOK, data.OK(user))
}

// UpdateProfile handles PATCH /api/user/profile.
// Accepts a partial JSON body — only the fields present are updated.
func (uc *UserController) UpdateProfile(c *gin.Context) {
	userID := mustUserID(c)

	var req updateProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, data.Fail(err.Error()))
		return
	}

	if req.Name == nil && req.Email == nil && req.Bio == nil {
		c.JSON(http.StatusBadRequest, data.Fail("provide at least one field to update"))
		return
	}

	user, err := uc.svc.UpdateProfile(c.Request.Context(), userID, req.Name, req.Email, req.Bio)
	if err != nil {
		utils.Error(constants.LogTagUser, "UpdateProfile failed for "+userID, err)
		c.JSON(appErrStatus(err), data.Fail(appErrMsg(err)))
		return
	}

	utils.Info(constants.LogTagUser, "profile updated for user "+userID)
	c.JSON(http.StatusOK, data.OK(user))
}

// UpdateAvatar handles PATCH /api/user/avatar.
// Accepts a pre-signed / external avatar_url and stores it on the user record.
// Real file uploads would be handled by object storage (S3, GCS, etc.) on the
// client side; this endpoint stores the resulting URL only.
func (uc *UserController) UpdateAvatar(c *gin.Context) {
	userID := mustUserID(c)

	var req updateAvatarRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, data.Fail(err.Error()))
		return
	}

	user, err := uc.svc.UpdateAvatar(c.Request.Context(), userID, req.AvatarURL)
	if err != nil {
		utils.Error(constants.LogTagUser, "UpdateAvatar failed for "+userID, err)
		c.JSON(appErrStatus(err), data.Fail(appErrMsg(err)))
		return
	}

	utils.Info(constants.LogTagUser, "avatar updated for user "+userID)
	c.JSON(http.StatusOK, data.OK(user))
}

// ── Private helpers ───────────────────────────────────────────────────────────

// mustUserID extracts the authenticated user ID set by the JWT middleware.
// Panics if the value is missing — this should never happen on a protected route.
func mustUserID(c *gin.Context) string {
	val, _ := c.Get(string(constants.ContextKeyUserID))
	return val.(string)
}
