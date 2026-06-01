package services

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"strings"
	"time"

	"service-controller-notebookum/internal/transport/upstream"
)

type ExtractResponse struct {
	Text      string `json:"text"`
	PageCount int    `json:"page_count,omitempty"`
	Status    string `json:"status,omitempty"`
}

type ExtractService struct {
	client *upstream.Client
}

func NewExtractService(baseURL string, timeout time.Duration) *ExtractService {
	return &ExtractService{
		client: upstream.New(baseURL, timeout),
	}
}

// Extract envía un archivo PDF y extrae texto
func (s *ExtractService) Extract(ctx context.Context, filename string, fileContent []byte, headers http.Header) (*ExtractResponse, error) {
	// Crear multipart request manualmente
	status, body, _, err := s.extractViaMultipart(filename, fileContent, headers)
	if err != nil {
		return nil, err
	}

	if status != http.StatusOK && status != http.StatusAccepted {
		return nil, errors.New("extract service returned error")
	}

	var resp ExtractResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, err
	}

	return &resp, nil
}

// extractViaMultipart envía el archivo como multipart/form-data
func (s *ExtractService) extractViaMultipart(filename string, fileContent []byte, headers http.Header) (int, []byte, http.Header, error) {
	// Crear un cliente HTTP personalizado para esta solicitud
	client := &http.Client{
		Timeout: s.client.Timeout,
	}

	// Construir el URL
	target := strings.TrimRight(s.client.BaseURL, "/") + "/api/v1/extract"

	// Crear multipart request
	pr, pw := io.Pipe()
	writer := multipart.NewWriter(pw)

	// Escribir el archivo en una goroutine
	go func() {
		defer pw.Close()
		fw, err := writer.CreateFormFile("file", filename)
		if err != nil {
			pw.CloseWithError(err)
			return
		}
		if _, err := fw.Write(fileContent); err != nil {
			pw.CloseWithError(err)
			return
		}
		writer.Close()
	}()

	req, err := http.NewRequest(http.MethodPost, target, pr)
	if err != nil {
		pr.Close()
		return 0, nil, nil, err
	}

	req.Header.Set("Content-Type", writer.FormDataContentType())

	// Copiar headers existentes
	if headers != nil {
		for key, values := range headers {
			for _, value := range values {
				req.Header.Add(key, value)
			}
		}
	}

	resp, err := client.Do(req)
	if err != nil {
		return 0, nil, nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return resp.StatusCode, nil, resp.Header.Clone(), err
	}

	return resp.StatusCode, body, resp.Header.Clone(), nil
}
