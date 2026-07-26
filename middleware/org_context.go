package middleware

import (
	"github.com/gin-gonic/gin"

	"syncspace-backend/constants"
	"syncspace-backend/service/database"
)

// OrgContext is a Gin middleware that resolves the authenticated user's active
// org membership and injects orgID + role into the request context.
//
// Must run after JWTAuth (requires ContextKeyUserID to be set).
// Does NOT abort on missing org — onboarding users have no org yet.
func OrgContext(repo *database.OrgRepo) gin.HandlerFunc {
	return func(c *gin.Context) {
		val, exists := c.Get(string(constants.ContextKeyUserID))
		if !exists {
			c.Next()
			return
		}
		userID := val.(string)

		orgID, role, found, err := repo.GetUserPrimaryOrgID(c.Request.Context(), userID)
		if err == nil && found {
			c.Set(string(constants.ContextKeyOrgID), orgID)
			c.Set(string(constants.ContextKeyOrgRole), role)
		}
		c.Next()
	}
}
