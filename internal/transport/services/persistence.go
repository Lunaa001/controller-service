package services

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"service-controller-notebookum/internal/transport/upstream"
)

type Document struct {
	ID       string `json:"id"`
	UserID   string `json:"user_id"`
	Text     string `json:"text,omitempty"`
	Summary  string `json:"summary,omitempty"`
	Status   string `json:"status"`
	Filename string `json:"filename,omitempty"`
}

type PersistenceService struct {
	client *upstream.Client
}

func NewPersistenceService(baseURL string, timeout time.Duration) *PersistenceService {
	return &PersistenceService{
		client: upstream.New(baseURL, timeout),
	}
}

// SaveDocument guarda un documento con texto extraído
func (s *PersistenceService) SaveDocument(ctx context.Context, doc *Document, headers http.Header) error {
	payload, _ := json.Marshal(doc)

	status, _, _, err := s.client.Request(http.MethodPost, "/api/v1/documents", payload, headers)
	if err != nil {
		return err
	}

	if status != http.StatusCreated && status != http.StatusOK {
		return errors.New("persistence service returned error")
	}

	return nil
}

// UpdateDocument actualiza un documento (ej: agregar resumen)
func (s *PersistenceService) UpdateDocument(ctx context.Context, docID string, doc *Document, headers http.Header) error {
	payload, _ := json.Marshal(doc)

	status, _, _, err := s.client.Request(http.MethodPut, "/api/v1/documents/"+docID, payload, headers)
	if err != nil {
		return err
	}

	if status != http.StatusOK && status != http.StatusNoContent {
		return errors.New("persistence service returned error")
	}

	return nil
}

// GetDocument obtiene un documento por ID
func (s *PersistenceService) GetDocument(ctx context.Context, docID string, headers http.Header) (*Document, error) {
	status, body, _, err := s.client.Request(http.MethodGet, "/api/v1/documents/"+docID, nil, headers)
	if err != nil {
		return nil, err
	}

	if status == http.StatusNotFound {
		return nil, errors.New("document not found")
	}

	if status != http.StatusOK {
		return nil, errors.New("persistence service returned error")
	}

	var doc Document
	if err := json.Unmarshal(body, &doc); err != nil {
		return nil, err
	}

	return &doc, nil
}

// SaveExtractedText guarda texto extraído de un documento
func (s *PersistenceService) SaveExtractedText(ctx context.Context, docID string, text string, headers http.Header) error {
	payload, _ := json.Marshal(map[string]string{
		"text":   text,
		"status": "extracted",
	})

	status, _, _, err := s.client.Request(http.MethodPatch, "/api/v1/documents/"+docID+"/text", payload, headers)
	if err != nil {
		return err
	}

	if status != http.StatusOK && status != http.StatusNoContent {
		return errors.New("persistence service returned error")
	}

	return nil
}

// SaveSummary guarda un resumen de un documento
func (s *PersistenceService) SaveSummary(ctx context.Context, docID string, summary string, headers http.Header) error {
	payload, _ := json.Marshal(map[string]string{
		"summary": summary,
		"status":  "ready",
	})

	status, _, _, err := s.client.Request(http.MethodPatch, "/api/v1/documents/"+docID+"/summary", payload, headers)
	if err != nil {
		return err
	}

	if status != http.StatusOK && status != http.StatusNoContent {
		return errors.New("persistence service returned error")
	}

	return nil
}
