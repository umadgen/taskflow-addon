package api_test

import (
	"net/http"
	"testing"

	"foyer/taskflow/internal/model"
)

func TestSettingsGetAndPatch(t *testing.T) {
	h := newTestHandler(t)

	resp := doReq(t, h, http.MethodGet, "/api/settings", "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("get: %d", resp.StatusCode)
	}
	var s model.Settings
	decodeJSON(t, resp, &s)
	if s.Theme == "" {
		t.Fatal("expected default theme")
	}

	resp = doReq(t, h, http.MethodPatch, "/api/settings", `{"householdName":"Maison Martin"}`)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("patch: %d", resp.StatusCode)
	}

	resp = doReq(t, h, http.MethodGet, "/api/settings", "")
	decodeJSON(t, resp, &s)
	if s.HouseholdName != "Maison Martin" {
		t.Fatalf("householdName not updated: %s", s.HouseholdName)
	}
}
