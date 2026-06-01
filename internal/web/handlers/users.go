package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"service-controller-notebookum/internal/config"
	"service-controller-notebookum/internal/transport/upstream"
	"service-controller-notebookum/internal/web/middleware"
	"service-controller-notebookum/internal/web/problem"
	"service-controller-notebookum/internal/web/validators"

	"github.com/gin-gonic/gin"
)

type UsersHandler struct {
	client *upstream.Client
}

func NewUsersHandler(cfg config.Config) *UsersHandler {
	return &UsersHandler{
		client: upstream.New(cfg.UserServiceURL, 5*time.Second),
	}
}

func (h *UsersHandler) Create(c *gin.Context) {
	userID := c.GetString("user_id")
	if userID == "" {
		userID = middleware.ExtractUserID(c)
	}

	var payload struct {
		Name  string `json:"name"`
		Email string `json:"email"`
	}
	if err := c.ShouldBindJSON(&payload); err != nil || !validators.ValidateUserCreationInput(payload.Name, payload.Email) {
		problem.Write(c, http.StatusBadRequest, "Bad Request", "Missing or invalid field", middleware.CorrelationID(c))
		return
	}

	reqBody, _ := json.Marshal(map[string]string{
		"name":  payload.Name,
		"email": payload.Email,
	})

	headers := c.Request.Header.Clone()
	headers.Set("X-User-ID", userID)

	status, body, respHeaders, err := h.client.Request(http.MethodPost, "/api/v1/users", reqBody, headers)
	if err != nil {
		problem.Write(c, http.StatusBadGateway, "Bad Gateway", "Service unavailable", middleware.CorrelationID(c))
		return
	}

	correlation := respHeaders.Get("X-Correlation-ID")
	if correlation != "" {
		c.Writer.Header().Set("X-Correlation-ID", correlation)
	}
	c.Data(status, "application/json", body)
}

func (h *UsersHandler) Get(c *gin.Context) {
	pathID := c.Param("id")
	userID := c.GetString("user_id")
	if userID != pathID {
		problem.Write(c, http.StatusForbidden, "Forbidden", "Access denied", middleware.CorrelationID(c))
		return
	}

	headers := c.Request.Header.Clone()
	headers.Set("X-User-ID", userID)

	status, body, _, err := h.client.Request(http.MethodGet, "/api/v1/users/"+pathID, nil, headers)
	if err != nil {
		problem.Write(c, http.StatusBadGateway, "Bad Gateway", "Service unavailable", middleware.CorrelationID(c))
		return
	}

	c.Data(status, "application/json", body)
}
