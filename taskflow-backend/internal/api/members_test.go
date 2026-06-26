package api_test

import (
	"net/http"
	"testing"

	"foyer/taskflow/internal/model"
)

func TestMembersLifecycle(t *testing.T) {
	h := newTestHandler(t)

	// Créer
	resp := doReq(t, h, http.MethodPost, "/api/members",
		`{"id":"m1","name":"Alice","initial":"A","tone":"rose"}`)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("create: expected 200, got %d", resp.StatusCode)
	}

	// Lister
	resp = doReq(t, h, http.MethodGet, "/api/members", "")
	var members []model.Member
	decodeJSON(t, resp, &members)
	if len(members) != 1 || members[0].Name != "Alice" {
		t.Fatalf("list: unexpected %+v", members)
	}

	// Mettre à jour
	resp = doReq(t, h, http.MethodPatch, "/api/members/m1", `{"name":"Alicia"}`)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("update: expected 200, got %d", resp.StatusCode)
	}
	resp = doReq(t, h, http.MethodGet, "/api/members", "")
	decodeJSON(t, resp, &members)
	if members[0].Name != "Alicia" {
		t.Fatalf("update: name not changed, got %s", members[0].Name)
	}

	// Supprimer
	resp = doReq(t, h, http.MethodDelete, "/api/members/m1", "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("delete: expected 200, got %d", resp.StatusCode)
	}
	resp = doReq(t, h, http.MethodGet, "/api/members", "")
	decodeJSON(t, resp, &members)
	if len(members) != 0 {
		t.Fatalf("delete: expected empty, got %d", len(members))
	}
}

func TestCreateMemberGeneratesID(t *testing.T) {
	h := newTestHandler(t)
	resp := doReq(t, h, http.MethodPost, "/api/members",
		`{"name":"Bob","initial":"B","tone":"sky"}`)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var result struct {
		Member model.Member `json:"member"`
	}
	decodeJSON(t, resp, &result)
	if result.Member.ID == "" {
		t.Fatal("expected generated ID")
	}
}
