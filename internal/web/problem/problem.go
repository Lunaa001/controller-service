package problem

import (
	"encoding/json"
	"strings"

	"github.com/gin-gonic/gin"
)

type Details struct {
	Type     string `json:"type"`
	Title    string `json:"title"`
	Status   int    `json:"status"`
	Detail   string `json:"detail"`
	Instance string `json:"instance,omitempty"`
}

func Write(c *gin.Context, status int, title, detail, requestID string) {
	body := Details{
		Type:     "https://api.universidad.localhost/errors/" + slug(title),
		Title:    title,
		Status:   status,
		Detail:   detail,
		Instance: "/api/v1?requestId=" + requestID,
	}

	raw, err := json.Marshal(body)
	if err != nil {
		c.Status(500)
		return
	}

	c.Data(status, "application/problem+json", raw)
}

func slug(title string) string {
	return strings.ToLower(strings.ReplaceAll(title, " ", "-"))
}
