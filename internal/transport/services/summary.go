package services

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"service-controller-notebookum/internal/transport/upstream"
)

type SummaryResponse struct {
	DocumentID string `json:"document_id,omitempty"`
	Filename   string `json:"filename,omitempty"`
	JobID      string `json:"job_id,omitempty"`
	Summary    string `json:"summary"`
	Modelo     string `json:"modelo,omitempty"`
	Status     string `json:"status,omitempty"`
}

type SummaryService struct {
	client *upstream.Client
}

func NewSummaryService(baseURL string, timeout time.Duration) *SummaryService {
	return &SummaryService{
		client: upstream.New(baseURL, timeout),
	}
}

// Generate genera un resumen a partir de texto extraído.
// Envía document_id, filename y job_id para tracking.
func (s *SummaryService) Generate(ctx context.Context, text string, documentID string, filename string, headers http.Header) (*SummaryResponse, error) {
	payload, _ := json.Marshal(map[string]interface{}{
		"document_id": documentID,
		"texto":       sanitizeUTF8(text),
		"filename":    filename,
		"max_tokens":  2048,
	})

	if headers == nil {
		headers = make(http.Header)
	} else {
		headers = headers.Clone()
	}
	headers.Set("Content-Type", "application/json")

	status, body, _, err := s.client.Request(http.MethodPost, "/summaries/generate", payload, headers)
	if err != nil {
		return nil, err
	}

	if status != http.StatusOK && status != http.StatusCreated {
		return nil, errors.New("summary service returned error: status " + http.StatusText(status) + " body: " + string(body))
	}

	// Parse standardized response: {"document_id": "...", "summary": "...", ...}
	var resp SummaryResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, err
	}

	if resp.Summary == "" {
		return nil, errors.New("no summary in response from summary service")
	}

	resp.Status = "ready"
	return &resp, nil
}
