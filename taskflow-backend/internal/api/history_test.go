package api_test

import (
	"net/http"
	"testing"

	"foyer/taskflow/internal/model"
)

func TestListHistory(t *testing.T) {
	h := newTestHandler(t)
	doReq(t, h, http.MethodPost, "/api/tasks",
		`{"id":"t1","title":"Vaisselle","cat":"maison","due":"2026-06-27T18:00","recurring":false}`)
	doReq(t, h, http.MethodPost, "/api/members",
		`{"id":"m1","name":"Alice","initial":"A","tone":"rose"}`)
	doReq(t, h, http.MethodPost, "/api/tasks/t1/complete",
		`{"memberId":"m1","histId":"h1","at":"2026-06-27T19:00:00Z"}`)

	resp := doReq(t, h, http.MethodGet, "/api/history", "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var hist []model.HistoryEntry
	decodeJSON(t, resp, &hist)
	if len(hist) != 1 || hist[0].ID != "h1" {
		t.Fatalf("unexpected history: %+v", hist)
	}
}
