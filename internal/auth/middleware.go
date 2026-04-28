package auth

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

const userIDContextKey = "user_id"

// AuthMiddleware validates JWT bearer tokens and injects the user ID into Gin context.
func AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		tokenString := extractBearerToken(c.GetHeader("Authorization"))
		if tokenString == "" {
			// Browser WebSocket API cannot set Authorization; allow query token on upgrade only.
			path := strings.TrimSuffix(c.Request.URL.Path, "/")
			if c.Request.Method == http.MethodGet && strings.HasSuffix(path, "/ws") {
				tokenString = strings.TrimSpace(c.Query("token"))
			}
		}
		if tokenString == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing authorization header"})
			return
		}

		claims, err := ValidateToken(tokenString)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid or expired token"})
			return
		}

		c.Set(userIDContextKey, claims.UserID)
		c.Next()
	}
}

func extractBearerToken(authHeader string) string {
	authHeader = strings.TrimSpace(authHeader)
	const bearerPrefix = "Bearer "
	if authHeader == "" || !strings.HasPrefix(authHeader, bearerPrefix) {
		return ""
	}
	return strings.TrimSpace(strings.TrimPrefix(authHeader, bearerPrefix))
}

// UserIDFromContext returns the authenticated user ID from Gin context.
func UserIDFromContext(c *gin.Context) (string, bool) {
	userID, ok := c.Get(userIDContextKey)
	if !ok {
		return "", false
	}

	value, ok := userID.(string)
	return value, ok
}
