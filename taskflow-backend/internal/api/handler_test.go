package api_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	chi "github.com/go-chi/chi/v5"

	"foyer/taskflow/internal/api"
	"foyer/taskflow/internal/db"
	"foyer/taskflow/internal/model"
	"foyer/taskflow/internal/ws"
)

func newTestHandler(t *testing.T) *api.Handler {
	t.Helper()
	d, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	hub := ws.NewHub()
	return api.NewHandler(d, hub, nil, "", "test-secret")
}

func doReq(t *testing.T, h *api.Handler, method, path, body string) *http.Response {
	t.Helper()
	r := chi.NewRouter()
	h.Mount(r)
	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)

	var bodyReader io.Reader
	if body != "" {
		bodyReader = strings.NewReader(body)
	}
	req, err := http.NewRequest(method, srv.URL+path, bodyReader)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	return resp
}

func decodeJSON(t *testing.T, resp *http.Response, v any) {
	t.Helper()
	defer resp.Body.Close()
	if err := json.NewDecoder(resp.Body).Decode(v); err != nil {
		t.Fatalf("decode: %v", err)
	}
}

func createTestServer(t *testing.T, h *api.Handler) *httptest.Server {
	t.Helper()
	r := chi.NewRouter()
	h.Mount(r)
	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)
	return srv
}

func TestGetSnapshot(t *testing.T) {
	h := newTestHandler(t)
	resp := doReq(t, h, http.MethodGet, "/api/snapshot", "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var result struct {
		Data model.AppData `json:"data"`
		Seq  int           `json:"seq"`
	}
	decodeJSON(t, resp, &result)
	if result.Data.Members == nil {
		t.Fatal("members should not be nil")
	}
	if result.Seq != 0 {
		t.Fatalf("expected seq=0, got %d", result.Seq)
	}
}
