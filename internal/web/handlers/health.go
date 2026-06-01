package handlers

import (
	"net/http"

	"service-controller-notebookum/internal/core/resilience"

	"github.com/gin-gonic/gin"
)

type HealthHandler struct {
	registry *resilience.Registry
}

func NewHealthHandler(registry *resilience.Registry) *HealthHandler {
	return &HealthHandler{registry: registry}
}

func (h *HealthHandler) Health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (h *HealthHandler) Ready(c *gin.Context) {
	openCircuits := h.registry.OpenServices()
	if len(openCircuits) > 0 {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"status":        "degraded",
			"open_circuits": openCircuits,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":    "ok",
		"upstreams": h.registry.Services(),
	})
}

func (h *HealthHandler) CircuitStatus(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"circuits": h.registry.Snapshot(),
	})
}
