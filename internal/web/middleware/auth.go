package middleware

import (
	"strings"

	"service-controller-notebookum/internal/web/problem"

	"github.com/gin-gonic/gin"
)

func ExtractUserID(c *gin.Context) string {
	if userID := c.GetHeader("X-User-ID"); userID != "" {
		return userID
	}

	auth := c.GetHeader("Authorization")
	if strings.HasPrefix(auth, "Bearer ") {
		return strings.TrimPrefix(auth, "Bearer ")
	}

	return ""
}

func RequireAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := ExtractUserID(c)
		if userID == "" {
			problem.Write(c, 401, "Unauthorized", "Missing or invalid authentication", CorrelationID(c))
			c.Abort()
			return
		}

		c.Set("user_id", userID)
		c.Next()
	}
}
