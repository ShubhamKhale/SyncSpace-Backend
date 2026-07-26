package controller

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"

	"syncspace-backend/constants"
	pkgcloudinary "syncspace-backend/pkg/cloudinary"
	"syncspace-backend/pkg/utils"
	"syncspace-backend/service"
	"syncspace-backend/service/data"
)

// UserController handles user profile endpoints.
type UserController struct {
	svc      *service.UserService
	uploader *pkgcloudinary.Uploader // nil when Cloudinary is not configured
}

// NewUserController creates a UserController backed by the given UserService.
// uploader may be nil — avatar file upload returns 503 in that case.
func NewUserController(svc *service.UserService, uploader *pkgcloudinary.Uploader) *UserController {
	return &UserController{svc: svc, uploader: uploader}
}

// ── Request types ─────────────────────────────────────────────────────────────

// updateProfileRequest uses pointer fields so PATCH can distinguish between
// "field not provided" (nil) and "field set to empty string" (non-nil, "").
type updateProfileRequest struct {
	Name  *string `json:"name"`
	Email *string `json:"email"  binding:"omitempty,email"`
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
		fmt.Println("GetMe error for user " + userID + ": " + err.Error())
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

	if req.Name == nil && req.Email == nil {
		c.JSON(http.StatusBadRequest, data.Fail("provide at least one field to update"))
		return
	}

	user, err := uc.svc.UpdateProfile(c.Request.Context(), userID, req.Name, req.Email)
	if err != nil {
		utils.Error(constants.LogTagUser, "UpdateProfile failed for "+userID, err)
		c.JSON(appErrStatus(err), data.Fail(appErrMsg(err)))
		return
	}

	utils.Info(constants.LogTagUser, "profile updated for user "+userID)
	c.JSON(http.StatusOK, data.OK(user))
}

// UploadAvatar handles POST /api/user/avatar.
// Accepts multipart/form-data with an "avatar" field (PNG/JPG, max 1 MB).
// Uploads to Cloudinary and saves the resulting URL on the user record.
func (uc *UserController) UploadAvatar(c *gin.Context) {
	if uc.uploader == nil {
		c.JSON(http.StatusServiceUnavailable, data.Fail("avatar upload is not configured"))
		return
	}

	userID := mustUserID(c)

	file, header, err := c.Request.FormFile("avatar")
	if err != nil {
		c.JSON(http.StatusBadRequest, data.Fail("avatar file is required (field name: avatar)"))
		return
	}
	defer file.Close()

	secureURL, err := uc.uploader.UploadAvatar(c.Request.Context(), file, header, userID)
	if err != nil {
		utils.Error(constants.LogTagUser, "UploadAvatar cloudinary failed for "+userID, err)
		c.JSON(http.StatusBadRequest, data.Fail(err.Error()))
		return
	}

	user, err := uc.svc.UpdateAvatar(c.Request.Context(), userID, secureURL)
	if err != nil {
		utils.Error(constants.LogTagUser, "UploadAvatar save failed for "+userID, err)
		c.JSON(appErrStatus(err), data.Fail(appErrMsg(err)))
		return
	}

	utils.Info(constants.LogTagUser, "avatar uploaded for user "+userID)
	c.JSON(http.StatusOK, data.OK(map[string]string{"avatar_url": user.AvatarURL}))
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

// GetNotificationPrefs handles GET /api/settings/notifications.
func (uc *UserController) GetNotificationPrefs(c *gin.Context) {
	userID := mustUserID(c)

	prefs, err := uc.svc.GetNotificationPrefs(c.Request.Context(), userID)
	if err != nil {
		utils.Error(constants.LogTagUser, "GetNotificationPrefs failed for "+userID, err)
		c.JSON(appErrStatus(err), data.Fail(appErrMsg(err)))
		return
	}

	c.JSON(http.StatusOK, data.OK(prefs))
}

// UpdateNotificationPrefs handles PATCH /api/settings/notifications.
func (uc *UserController) UpdateNotificationPrefs(c *gin.Context) {
	userID := mustUserID(c)

	var req struct {
		Comments       *bool `json:"comments"`
		Invites        *bool `json:"invites"`
		ProductUpdates *bool `json:"product_updates"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, data.Fail("invalid preferences payload"))
		return
	}
	if req.Comments == nil && req.Invites == nil && req.ProductUpdates == nil {
		c.JSON(http.StatusBadRequest, data.Fail("provide at least one preference to update"))
		return
	}

	// Load current prefs so unspecified fields keep their existing values.
	current, err := uc.svc.GetNotificationPrefs(c.Request.Context(), userID)
	if err != nil {
		utils.Error(constants.LogTagUser, "UpdateNotificationPrefs fetch failed for "+userID, err)
		c.JSON(appErrStatus(err), data.Fail(appErrMsg(err)))
		return
	}

	if req.Comments != nil {
		current.Comments = *req.Comments
	}
	if req.Invites != nil {
		current.Invites = *req.Invites
	}
	if req.ProductUpdates != nil {
		current.ProductUpdates = *req.ProductUpdates
	}

	prefs, err := uc.svc.UpdateNotificationPrefs(c.Request.Context(), userID, current)
	if err != nil {
		utils.Error(constants.LogTagUser, "UpdateNotificationPrefs failed for "+userID, err)
		c.JSON(appErrStatus(err), data.Fail(appErrMsg(err)))
		return
	}

	utils.Info(constants.LogTagUser, "notification prefs updated for user "+userID)
	c.JSON(http.StatusOK, data.OK(prefs))
}

// ── Private helpers ───────────────────────────────────────────────────────────

// mustUserID extracts the authenticated user ID set by the JWT middleware.
// Panics if the value is missing — this should never happen on a protected route.
func mustUserID(c *gin.Context) string {
	val, _ := c.Get(string(constants.ContextKeyUserID))
	return val.(string)
}
