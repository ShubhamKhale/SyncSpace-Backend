package middleware

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"

	"syncspace-backend/constants"
	"syncspace-backend/service/data"
)

// JWTAuth returns a Gin middleware that validates a Bearer JWT in the
// Authorization header and injects the authenticated user's ID into the
// Gin context under constants.ContextKeyUserID.
//
// Downstream handlers (and the encryption middleware) retrieve the user ID via:
//
//	val, _ := c.Get(string(constants.ContextKeyUserID))
func JWTAuth(jwtSecret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		fmt.Println("Authorization header:", authHeader)
		if !strings.HasPrefix(authHeader, "Bearer ") {
			c.AbortWithStatusJSON(http.StatusUnauthorized,
				data.Fail("missing or invalid Authorization header"))
			return
		}

		tokenStr := strings.TrimPrefix(authHeader, "Bearer ")
		fmt.Println("Extracted token:", tokenStr)

		token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (any, error) {
			if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, jwt.ErrSignatureInvalid
			}
			return []byte(jwtSecret), nil
		}, jwt.WithValidMethods([]string{"HS256"}))

		if err != nil || !token.Valid {
			fmt.Println("Token error:", err)
			c.AbortWithStatusJSON(http.StatusUnauthorized,
				data.Fail("invalid or expired token"))
			return
		}

		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			fmt.Println("Malformed token claims")
			c.AbortWithStatusJSON(http.StatusUnauthorized,
				data.Fail("malformed token claims"))
			return
		}

		userID, ok := claims["sub"].(string)
		if !ok || userID == "" {
			fmt.Println("Token missing subject")
			c.AbortWithStatusJSON(http.StatusUnauthorized,
				data.Fail("token missing subject"))
			return
		}
		fmt.Println("Authenticated user ID:", userID)

		// Make user_id available to all downstream handlers and middleware.
		c.Set(string(constants.ContextKeyUserID), userID)
		c.Next()
	}
}
