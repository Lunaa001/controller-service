package handlers

import (
	"net/http"

	"service-controller-notebookum/internal/core/orchestrator"
	"service-controller-notebookum/internal/web/middleware"
	"service-controller-notebookum/internal/web/problem"

	"github.com/gin-gonic/gin"
)

type SummariesHandler struct {
	orchestrator *orchestrator.Orchestrator
}

func NewSummariesHandler(orch *orchestrator.Orchestrator) *SummariesHandler {
	return &SummariesHandler{orchestrator: orch}
}

// Get obtiene el resumen de un documento
func (h *SummariesHandler) Get(c *gin.Context) {
	documentID := c.Param("id")
	userID := c.GetString("user_id")
	correlationID := middleware.CorrelationID(c)

	result, err := h.orchestrator.GetDocument(
		c.Request.Context(),
		documentID,
		userID,
		correlationID,
	)
	if err != nil {
		problem.Write(c, http.StatusNotFound, "Not Found", "Document not found", correlationID)
		return
	}

	// Si el documento aún no tiene resumen, retornar 202 Accepted
	if result.Summary == "" {
		problem.Write(c, http.StatusAccepted, "Accepted", "Document still processing", correlationID)
		return
	}

	// Retornar el resumen completo
	c.JSON(http.StatusOK, gin.H{
		"document_id": result.DocumentID,
		"summary":     result.Summary,
		"status":      result.Status,
	})
}
