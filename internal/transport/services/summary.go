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
	Summary string `json:"summary"`
	Status  string `json:"status,omitempty"`
}

type SummaryService struct {
	client *upstream.Client
}

func NewSummaryService(baseURL string, timeout time.Duration) *SummaryService {
	return &SummaryService{
		client: upstream.New(baseURL, timeout),
	}
}

// Generate genera un resumen a partir de texto extraído
func (s *SummaryService) Generate(ctx context.Context, text string, headers http.Header) (*SummaryResponse, error) {
	// El Summary Service espera: {"documento_id": N, "texto": "...", "max_tokens": 300}
	payload, _ := json.Marshal(map[string]interface{}{
		"documento_id": 1,
		"texto":        sanitizeUTF8(text),
		"max_tokens":   500,
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

	// La respuesta viene como: {"success": true, "data": {"resumen": "..."}, ...}
	var raw map[string]interface{}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, err
	}

	// Extraer el resumen del campo data.resumen
	summary := ""
	if data, ok := raw["data"].(map[string]interface{}); ok {
		if resumen, ok := data["resumen"].(string); ok {
			summary = resumen
		}
	}

	if summary == "" {
		return nil, errors.New("no summary in response from summary service")
	}

	return &SummaryResponse{
		Summary: summary,
		Status:  "ready",
	}, nil
}

