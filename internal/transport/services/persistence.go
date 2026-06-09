package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"service-controller-notebookum/internal/transport/upstream"
)

// sanitizeUTF8 elimina caracteres nulos (0x00) que Postgres rechaza
func sanitizeUTF8(s string) string {
	return strings.ReplaceAll(s, "\x00", "")
}

type Document struct {
	ID          string `json:"id"`
	UserID      string `json:"user_id"`
	Text        string `json:"text,omitempty"`
	Summary     string `json:"summary,omitempty"`
	Status      string `json:"status"`
	Filename    string `json:"filename,omitempty"`
	ContentHash string `json:"content_hash,omitempty"`
}

type PersistenceService struct {
	client *upstream.Client
}

func NewPersistenceService(baseURL string, timeout time.Duration) *PersistenceService {
	return &PersistenceService{
		client: upstream.New(baseURL, timeout),
	}
}

// helper para asegurar JSON content type
func (s *PersistenceService) request(method, path string, payload []byte, headers http.Header) (int, []byte, http.Header, error) {
	if headers == nil {
		headers = make(http.Header)
	} else {
		headers = headers.Clone()
	}
	headers.Set("Content-Type", "application/json")
	return s.client.Request(method, path, payload, headers)
}

// GetDocumentByHash busca un documento por su hash SHA-256.
// Si lo encuentra, devuelve el documento con su resumen asociado.
// Si no existe, devuelve nil sin error.
func (s *PersistenceService) GetDocumentByHash(ctx context.Context, contentHash string, headers http.Header) (*Document, error) {
	status, body, _, err := s.request(http.MethodGet, "/api/v1/db/documents/hash/"+contentHash, nil, headers)
	if err != nil {
		return nil, err
	}

	if status == http.StatusNotFound {
		return nil, nil // No existe — no es error
	}

	if status != http.StatusOK {
		return nil, errors.New("persistence service returned error: status " + http.StatusText(status) + " body: " + string(body))
	}

	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}

	doc := &Document{}
	if id, ok := result["id"].(float64); ok {
		doc.ID = fmt.Sprintf("%.0f", id)
	}
	if val, ok := result["filename"].(string); ok {
		doc.Filename = val
	}
	if val, ok := result["status"].(string); ok {
		doc.Status = val
	}
	if val, ok := result["extractedText"].(string); ok {
		doc.Text = val
	}
	if val, ok := result["contentHash"].(string); ok {
		doc.ContentHash = val
	}

	return doc, nil
}

// GetSummaryByDocumentID obtiene el resumen asociado a un documento por su ID
func (s *PersistenceService) GetSummaryByDocumentID(ctx context.Context, docID string, headers http.Header) (string, error) {
	status, body, _, err := s.request(http.MethodGet, "/api/v1/db/documents/"+docID+"/summary", nil, headers)
	if err != nil {
		return "", err
	}

	if status == http.StatusNotFound {
		return "", nil
	}

	if status != http.StatusOK {
		return "", errors.New("persistence service returned error: status " + http.StatusText(status) + " body: " + string(body))
	}

	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", err
	}

	if content, ok := result["content"].(string); ok {
		return content, nil
	}

	return "", nil
}

// SaveDocument guarda un documento con texto extraído
func (s *PersistenceService) SaveDocument(ctx context.Context, doc *Document, headers http.Header) error {
	userIdInt, err := strconv.ParseInt(doc.UserID, 10, 64)
	if err != nil || userIdInt <= 0 {
		userIdInt = 1 // Fallback para "anonymous"
	}

	dto := map[string]interface{}{
		"userId":        userIdInt,
		"filename":      doc.Filename,
		"filePath":      "in-memory",
		"status":        doc.Status,
		"extractedText": sanitizeUTF8(doc.Text),
		"contentHash":   doc.ContentHash,
	}
	payload, _ := json.Marshal(dto)

	status, body, _, err := s.request(http.MethodPost, "/api/v1/db/documents", payload, headers)
	if err != nil {
		return err
	}

	if status != http.StatusCreated && status != http.StatusOK {
		return errors.New("persistence service returned error: status " + http.StatusText(status) + " body: " + string(body))
	}

	// Extraer el ID generado por la BD
	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err == nil {
		if id, ok := result["id"].(float64); ok {
			doc.ID = fmt.Sprintf("%.0f", id) // Actualizar ID en el doc para usarlo en SaveSummary
		}
	}

	return nil
}

// UpdateDocument actualiza un documento
func (s *PersistenceService) UpdateDocument(ctx context.Context, docID string, doc *Document, headers http.Header) error {
	dto := map[string]interface{}{
		"status": doc.Status,
	}
	payload, _ := json.Marshal(dto)

	status, body, _, err := s.request(http.MethodPatch, "/api/v1/db/documents/"+docID, payload, headers)
	if err != nil {
		return err
	}

	if status != http.StatusOK && status != http.StatusNoContent {
		return errors.New("persistence service returned error: status " + http.StatusText(status) + " body: " + string(body))
	}

	return nil
}

// GetDocument obtiene un documento por ID
func (s *PersistenceService) GetDocument(ctx context.Context, docID string, headers http.Header) (*Document, error) {
	status, body, _, err := s.request(http.MethodGet, "/api/v1/db/documents/"+docID, nil, headers)
	if err != nil {
		return nil, err
	}

	if status == http.StatusNotFound {
		return nil, errors.New("document not found")
	}

	if status != http.StatusOK {
		return nil, errors.New("persistence service returned error: status " + http.StatusText(status) + " body: " + string(body))
	}

	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}

	doc := &Document{
		ID: docID,
	}
	
	if val, ok := result["filename"].(string); ok {
		doc.Filename = val
	}
	if val, ok := result["status"].(string); ok {
		doc.Status = val
	}
	if val, ok := result["extractedText"].(string); ok {
		doc.Text = val
	}

	return doc, nil
}

// SaveSummary guarda un resumen de un documento
func (s *PersistenceService) SaveSummary(ctx context.Context, docID string, summary string, headers http.Header) error {
	docIdInt, err := strconv.Atoi(docID)
	if err != nil {
		return fmt.Errorf("invalid document ID for summary: %s", docID)
	}

	dto := map[string]interface{}{
		"documentId": docIdInt,
		"content":    sanitizeUTF8(summary),
		"modelUsed":  "llama-3.3-70b-versatile",
	}
	payload, _ := json.Marshal(dto)

	status, body, _, err := s.request(http.MethodPost, "/api/v1/db/summaries", payload, headers)
	if err != nil {
		return err
	}

	if status != http.StatusCreated && status != http.StatusOK {
		return errors.New("persistence service returned error: status " + http.StatusText(status) + " body: " + string(body))
	}

	return nil
}
