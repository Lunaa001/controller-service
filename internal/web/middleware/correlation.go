package middleware

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func Correlation(header string) gin.HandlerFunc {
	if header == "" {
		header = "X-Correlation-ID"
	}

	return func(c *gin.Context) {
		correlationID := c.GetHeader(header)
		if correlationID == "" {
			correlationID = uuid.NewString()
		}

		c.Set("correlation_id", correlationID)
		c.Writer.Header().Set(header, correlationID)
		c.Next()
	}
}

func CorrelationID(c *gin.Context) string {
	if value, ok := c.Get("correlation_id"); ok {
		if id, ok := value.(string); ok {
			return id
		}
	}
	return ""
}
