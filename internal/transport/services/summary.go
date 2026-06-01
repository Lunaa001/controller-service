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
	payload, _ := json.Marshal(map[string]string{
		"text": text,
	})

	status, body, _, err := s.client.Request(http.MethodPost, "/api/v1/summaries", payload, headers)
	if err != nil {
		return nil, err
	}

	if status != http.StatusOK && status != http.StatusAccepted {
		return nil, errors.New("summary service returned error")
	}

	var resp SummaryResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, err
	}

	return &resp, nil
}
