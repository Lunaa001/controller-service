package handlers

import (
	"io"
	"net/http"

	"service-controller-notebookum/internal/core/orchestrator"
	"service-controller-notebookum/internal/web/middleware"
	"service-controller-notebookum/internal/web/problem"
	"service-controller-notebookum/internal/web/validators"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type DocumentsHandler struct {
	orchestrator *orchestrator.Orchestrator
}

func NewDocumentsHandler(orch *orchestrator.Orchestrator) *DocumentsHandler {
	return &DocumentsHandler{orchestrator: orch}
}

// Upload maneja la carga de documentos y orquesta el flujo completo
func (h *DocumentsHandler) Upload(c *gin.Context) {
	file, err := c.FormFile("file")
	if err != nil {
		problem.Write(c, http.StatusBadRequest, "Bad Request", "Missing file field", middleware.CorrelationID(c))
		return
	}

	// Validar content type
	if !validators.ValidatePDFContentType(file.Header.Get("Content-Type")) {
		problem.Write(c, http.StatusBadRequest, "Bad Request", "Only PDF files are accepted (Content-Type must be application/pdf)", middleware.CorrelationID(c))
		return
	}

	src, err := file.Open()
	if err != nil {
		problem.Write(c, http.StatusBadRequest, "Bad Request", "Unable to read file", middleware.CorrelationID(c))
		return
	}
	defer src.Close()

	content, err := io.ReadAll(src)
	if err != nil {
		problem.Write(c, http.StatusBadRequest, "Bad Request", "Unable to read file", middleware.CorrelationID(c))
		return
	}

	// Validar tamaño máximo (25MB)
	if len(content) > 25*1024*1024 {
		problem.Write(c, http.StatusRequestEntityTooLarge, "Payload Too Large", "File exceeds maximum size of 25MB", middleware.CorrelationID(c))
		return
	}

	userID := c.GetString("user_id")
	correlationID := middleware.CorrelationID(c)
	documentID := uuid.NewString()

	// Orquestar el flujo completo: extract → summary → persist → cache
	result, err := h.orchestrator.ProcessDocument(
		c.Request.Context(),
		documentID,
		file.Filename,
		content,
		userID,
		correlationID,
	)
	if err != nil {
		problem.Write(c, http.StatusBadGateway, "Bad Gateway", "Failed to process document: "+err.Error(), correlationID)
		return
	}

	// Retornar respuesta exitosa
	c.JSON(http.StatusOK, gin.H{
		"document_id": result.DocumentID,
		"status":      result.Status,
		"summary":     result.Summary,
	})
}

// Status obtiene el estado de un documento
func (h *DocumentsHandler) Status(c *gin.Context) {
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

	code := http.StatusAccepted
	if result.Status == "ready" {
		code = http.StatusOK
	}

	c.JSON(code, gin.H{
		"document_id": result.DocumentID,
		"status":      result.Status,
	})
}
