package handlers

import (
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"strings"

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

// jsonUploadRequest represents a JSON upload with base64-encoded PDF
type jsonUploadRequest struct {
	FileName string `json:"file_name"`
	Content  string `json:"content"` // base64 encoded PDF
}

// Upload maneja la carga de documentos y orquesta el flujo completo.
// Acepta dos formatos:
//   - multipart/form-data con campo "file" (Postman, curl)
//   - application/json con { "file_name": "...", "content": "<base64>" } (Dashboard)
func (h *DocumentsHandler) Upload(c *gin.Context) {
	var (
		filename string
		content  []byte
		err      error
	)

	contentType := c.GetHeader("Content-Type")

	if strings.Contains(contentType, "application/json") {
		// ── JSON upload (Dashboard) ──────────────────────────────────
		var req jsonUploadRequest
		if err := json.NewDecoder(c.Request.Body).Decode(&req); err != nil {
			problem.Write(c, http.StatusBadRequest, "Bad Request", "Invalid JSON body", middleware.CorrelationID(c))
			return
		}

		if req.FileName == "" || req.Content == "" {
			problem.Write(c, http.StatusBadRequest, "Bad Request", "file_name and content (base64) are required", middleware.CorrelationID(c))
			return
		}

		filename = req.FileName
		content, err = base64.StdEncoding.DecodeString(req.Content)
		if err != nil {
			problem.Write(c, http.StatusBadRequest, "Bad Request", "Invalid base64 content", middleware.CorrelationID(c))
			return
		}

	} else {
		// ── Multipart upload (Postman, curl) ─────────────────────────
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

		content, err = io.ReadAll(src)
		if err != nil {
			problem.Write(c, http.StatusBadRequest, "Bad Request", "Unable to read file", middleware.CorrelationID(c))
			return
		}

		filename = file.Filename
	}

	// Validar que es un PDF (magic bytes: %PDF)
	if len(content) < 4 || string(content[:4]) != "%PDF" {
		problem.Write(c, http.StatusBadRequest, "Bad Request", "File is not a valid PDF", middleware.CorrelationID(c))
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
		filename,
		content,
		userID,
		correlationID,
	)
	if err != nil {
		problem.Write(c, http.StatusBadGateway, "Bad Gateway", "Failed to process document: "+err.Error(), correlationID)
		return
	}

	// Retornar respuesta exitosa con texto extraído y resumen
	c.JSON(http.StatusOK, gin.H{
		"document_id":    result.DocumentID,
		"status":         result.Status,
		"text":           result.Text,
		"extracted_text": result.Text,
		"summary":        result.Summary,
		"filename":       filename,
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
