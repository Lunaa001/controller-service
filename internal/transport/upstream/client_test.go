package upstream

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestRequestSuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Correlation-ID") != "cid1" {
			t.Fatalf("missing correlation id")
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":"u1"}`))
	}))
	defer server.Close()

	client := New(server.URL, time.Second)
	status, body, headers, err := client.Request(http.MethodPost, "/users", []byte(`{"name":"John"}`), http.Header{
		"X-Correlation-ID": []string{"cid1"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if status != http.StatusCreated {
		t.Fatalf("expected 201, got %d", status)
	}
	if string(body) != `{"id":"u1"}` {
		t.Fatalf("unexpected body %s", string(body))
	}
	if headers.Get("Content-Type") != "application/json" {
		t.Fatalf("expected content type, got %q", headers.Get("Content-Type"))
	}
}
