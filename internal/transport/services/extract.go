package services

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"service-controller-notebookum/internal/transport/upstream"
)

// ExtractResponse es la respuesta del microservicio de extracción
type ExtractResponse struct {
	Status  string `json:"status"`
	Message string `json:"message"`
	Content string `json:"content"`
	// adaptamos a la respuesta de extract-service
	Text string `json:"text,omitempty"`
}

type ExtractRequest struct {
	FileName string `json:"file_name"`
	Content  string `json:"content"`
	MaxPages *int   `json:"max_pages,omitempty"`
}

type ExtractService struct {
	client *upstream.Client
}

func NewExtractService(baseURL string, timeout time.Duration) *ExtractService {
	return &ExtractService{
		client: upstream.New(baseURL, timeout),
	}
}

// Extract envía un archivo PDF codificado en Base64 y extrae texto
func (s *ExtractService) Extract(ctx context.Context, filename string, fileContent []byte, headers http.Header) (*ExtractResponse, error) {
	// 1. Codificar contenido del PDF a Base64
	base64Content := base64.StdEncoding.EncodeToString(fileContent)

	// 2. Crear payload JSON
	reqBody := ExtractRequest{
		FileName: filename,
		Content:  base64Content,
	}
	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

	// 3. Crear HTTP request
	target := strings.TrimRight(s.client.BaseURL, "/") + "/api/v1/pdf/process"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, target, bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")

	// Copiar headers existentes
	if headers != nil {
		for key, values := range headers {
			for _, value := range values {
				req.Header.Add(key, value)
			}
		}
	}

	// 4. Enviar request
	client := &http.Client{
		Timeout: s.client.Timeout,
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
		return nil, errors.New("extract service returned error status: " + resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var extResp ExtractResponse
	if err := json.Unmarshal(body, &extResp); err != nil {
		return nil, err
	}

	// Como el extract-service devuelve {"content": "..."} pero el controller
	// originalmente esperaba {"text": "..."}, mapeamos `content` a `Text`
	if extResp.Text == "" && extResp.Content != "" {
		extResp.Text = extResp.Content
	}

	return &extResp, nil
}
