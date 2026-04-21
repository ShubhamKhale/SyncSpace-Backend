package controller

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"syncspace-backend/constants"
	"syncspace-backend/errs"
	"syncspace-backend/model"
	"syncspace-backend/pkg/utils"
	"syncspace-backend/service"
	"syncspace-backend/service/data"
)

// AuthController handles registration, sign-in, and logout endpoints.
type AuthController struct {
	svc *service.AuthService
}

// NewAuthController creates an AuthController backed by the given AuthService.
func NewAuthController(svc *service.AuthService) *AuthController {
	return &AuthController{svc: svc}
}

// ── Request / response types ──────────────────────────────────────────────────

type registerRequest struct {
	Name     string `json:"name"     binding:"required"`
	Email    string `json:"email"    binding:"required,email"`
	Password string `json:"password" binding:"required,min=8"`
}

type signInRequest struct {
	Email    string `json:"email"    binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

type authResponse struct {
	Token      string      `json:"token"`
	SessionKey string      `json:"session_key"` // base64 AES-256 key for payload encryption
	User       *model.User `json:"user"`
}

// ── Handlers ─────────────────────────────────────────────────────────────────

// Register handles POST /api/auth/register.
// Creates a new user account, hashes the password, and returns a JWT + session key.
func (ac *AuthController) Register(c *gin.Context) {
	var req registerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, data.Fail(err.Error()))
		return
	}

	token, sessionKey, user, err := ac.svc.Register(c.Request.Context(), req.Name, req.Email, req.Password)
	if err != nil {
		utils.Error(constants.LogTagAuth, "register failed", err)
		c.JSON(appErrStatus(err), data.Fail(appErrMsg(err)))
		return
	}

	utils.Info(constants.LogTagAuth, "user registered: "+user.ID)
	c.JSON(http.StatusCreated, data.OK(authResponse{
		Token:      token,
		SessionKey: sessionKey,
		User:       user,
	}))
}

// SignIn handles POST /api/auth/signin.
// Validates email/password against the database and returns a JWT + session key.
func (ac *AuthController) SignIn(c *gin.Context) {
	var req signInRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, data.Fail(err.Error()))
		return
	}

	token, sessionKey, user, err := ac.svc.SignIn(c.Request.Context(), req.Email, req.Password)
	if err != nil {
		utils.Error(constants.LogTagAuth, "sign-in failed: "+req.Email, err)
		// Always 401 — don't reveal whether the email exists.
		c.JSON(http.StatusUnauthorized, data.Fail("invalid email or password"))
		return
	}

	utils.Info(constants.LogTagAuth, "user signed in: "+user.ID)
	c.JSON(http.StatusOK, data.OK(authResponse{
		Token:      token,
		SessionKey: sessionKey,
		User:       user,
	}))
}

// Logout handles POST /api/auth/logout.
// Revokes the AES session key; the JWT continues to be valid until its expiry.
// Requires JWT middleware to run first (sets ContextKeyUserID in context).
func (ac *AuthController) Logout(c *gin.Context) {
	userID, exists := c.Get(string(constants.ContextKeyUserID))
	if !exists {
		c.JSON(http.StatusUnauthorized, data.Fail("authentication required"))
		return
	}
	ac.svc.RevokeSessionKey(userID.(string))
	utils.Info(constants.LogTagAuth, "session revoked for user: "+userID.(string))
	c.JSON(http.StatusOK, data.OK(gin.H{"message": "logged out"}))
}

// ── Helpers ───────────────────────────────────────────────────────────────────

// appErrStatus maps an *errs.AppError code to an HTTP status code.
func appErrStatus(err error) int {
	if e, ok := err.(*errs.AppError); ok {
		switch e.Code {
		case "BAD_REQUEST":
			return http.StatusBadRequest
		case "UNAUTHORIZED":
			return http.StatusUnauthorized
		case "NOT_FOUND":
			return http.StatusNotFound
		case "CONFLICT":
			return http.StatusConflict
		}
	}
	return http.StatusInternalServerError
}

// appErrMsg extracts the human-readable message from an *errs.AppError,
// falling back to a generic message for unknown error types.
func appErrMsg(err error) string {
	if e, ok := err.(*errs.AppError); ok {
		return e.Message
	}
	return "internal server error"
}
