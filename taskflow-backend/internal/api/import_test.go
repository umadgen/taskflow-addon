package api_test

import (
	"net/http"
	"strings"
	"testing"
)

func TestImportRejectsWithoutSecret(t *testing.T) {
	h := newTestHandler(t)
	body := `{"version":1,"members":[],"tasks":[],"history":[],"pets":[],"googleEvents":[],"settings":{"theme":"dark","accent":"#6366f1","notifTasks":false,"notifPets":false,"notifAgenda":false,"sounds":false,"currentMember":"","householdName":"Test","onboarded":false}}`
	resp := doReq(t, h, http.MethodPost, "/api/import", body)
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", resp.StatusCode)
	}
}

func TestImportPopulatesDB(t *testing.T) {
	h := newTestHandler(t)
	payload := `{"version":1,"members":[{"id":"m1","name":"Alice","initial":"A","tone":"rose"}],"tasks":[{"id":"t1","title":"Courses","cat":"courses","due":"2026-06-28T10:00","recurring":false,"done":false,"late":false}],"history":[],"pets":[],"googleEvents":[],"settings":{"theme":"dark","accent":"#6366f1","notifTasks":false,"notifPets":false,"notifAgenda":false,"sounds":false,"currentMember":"","householdName":"Import Test","onboarded":false}}`

	srv := createTestServer(t, h)
	req, err := http.NewRequest(http.MethodPost, srv.URL+"/api/import", strings.NewReader(payload))
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-foyer-secret", "test-secret")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	// Vérifier que les données sont en DB
	resp = doReq(t, h, http.MethodGet, "/api/members", "")
	var result []any
	decodeJSON(t, resp, &result)
	if len(result) != 1 {
		t.Fatalf("expected 1 member, got %d", len(result))
	}
}

func TestImportRejectsIfNotEmpty(t *testing.T) {
	h := newTestHandler(t)
	doReq(t, h, http.MethodPost, "/api/members",
		`{"id":"m1","name":"Alice","initial":"A","tone":"rose"}`)

	srv := createTestServer(t, h)
	payload := `{"version":1,"members":[],"tasks":[],"history":[],"pets":[],"googleEvents":[],"settings":{"theme":"dark","accent":"#000","notifTasks":false,"notifPets":false,"notifAgenda":false,"sounds":false,"currentMember":"","householdName":"","onboarded":false}}`
	req, err := http.NewRequest(http.MethodPost, srv.URL+"/api/import", strings.NewReader(payload))
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-foyer-secret", "test-secret")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("expected 409 Conflict, got %d", resp.StatusCode)
	}
}
