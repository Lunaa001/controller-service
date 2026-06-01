package web

import (
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"testing"

	"service-controller-notebookum/internal/config"

	"github.com/gin-gonic/gin"
)

func featureRouter(t *testing.T, userURL string) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	return NewRouter(config.Config{
		CorrelationHeader: "X-Correlation-ID",
		UserServiceURL:    userURL,
	})
}

func TestCreateUserRoutesToUpstream(t *testing.T) {
	t.Helper()
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("X-User-ID"); got != "creator" {
			t.Fatalf("expected user header, got %q", got)
		}
		_, _ = io.Copy(io.Discard, r.Body)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":"u1","name":"John"}`))
	}))
	defer upstream.Close()

	router := featureRouter(t, upstream.URL)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/users", bytes.NewBufferString(`{"name":"John","email":"john@example.com"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-User-ID", "creator")
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", resp.Code)
	}

	var body map[string]any
	if err := json.Unmarshal(resp.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["id"] != "u1" {
		t.Fatalf("unexpected body: %#v", body)
	}
}

func TestDocumentUploadAndStatus(t *testing.T) {
	router := featureRouter(t, "")

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	part, err := writer.CreatePart(textprotoMIMEHeader("file", "doc.pdf", "application/pdf"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write([]byte("%PDF-1.4 test")); err != nil {
		t.Fatal(err)
	}
	_ = writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/documento/upload", &buf)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("X-User-ID", "u1")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	if resp.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d", resp.Code)
	}

	var uploaded struct {
		DocumentID string `json:"document_id"`
	}
	if err := json.Unmarshal(resp.Body.Bytes(), &uploaded); err != nil {
		t.Fatal(err)
	}
	if uploaded.DocumentID == "" {
		t.Fatal("expected document id")
	}

	statusReq := httptest.NewRequest(http.MethodGet, "/api/v1/documents/"+uploaded.DocumentID+"/status", nil)
	statusReq.Header.Set("X-User-ID", "u1")
	statusResp := httptest.NewRecorder()
	router.ServeHTTP(statusResp, statusReq)
	if statusResp.Code != http.StatusAccepted {
		t.Fatalf("expected 202 for status, got %d", statusResp.Code)
	}

	summaryReq := httptest.NewRequest(http.MethodGet, "/api/v1/summaries/document/"+uploaded.DocumentID, nil)
	summaryReq.Header.Set("X-User-ID", "u1")
	summaryResp := httptest.NewRecorder()
	router.ServeHTTP(summaryResp, summaryReq)
	if summaryResp.Code != http.StatusAccepted {
		t.Fatalf("expected 202 for summary, got %d", summaryResp.Code)
	}
}

func textprotoMIMEHeader(field, filename, contentType string) textproto.MIMEHeader {
	h := make(textproto.MIMEHeader)
	h.Set("Content-Disposition", `form-data; name="`+field+`"; filename="`+filename+`"`)
	h.Set("Content-Type", contentType)
	return h
}
