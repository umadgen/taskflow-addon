package api_test

import (
	"net/http"
	"testing"

	"foyer/taskflow/internal/model"
)

func TestPetsLifecycle(t *testing.T) {
	h := newTestHandler(t)

	resp := doReq(t, h, http.MethodPost, "/api/pets",
		`{"id":"p1","name":"Rex","species":"chien","emoji":"🐕","tone":"amber","breed":"Labrador","age":"3 ans","weight":30,"sex":"M","vaccines":[],"vet":[],"treatments":[],"weightLog":[]}`)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("create: %d", resp.StatusCode)
	}

	resp = doReq(t, h, http.MethodGet, "/api/pets", "")
	var pets []model.Pet
	decodeJSON(t, resp, &pets)
	if len(pets) != 1 || pets[0].Name != "Rex" {
		t.Fatalf("list: %+v", pets)
	}

	resp = doReq(t, h, http.MethodPatch, "/api/pets/p1", `{"name":"Rex Jr"}`)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("patch: %d", resp.StatusCode)
	}

	resp = doReq(t, h, http.MethodDelete, "/api/pets/p1", "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("delete: %d", resp.StatusCode)
	}
}

func TestCompleteTreatment(t *testing.T) {
	h := newTestHandler(t)
	doReq(t, h, http.MethodPost, "/api/pets",
		`{"id":"p1","name":"Rex","species":"chien","emoji":"🐕","tone":"amber","breed":"","age":"","weight":0,"sex":"","vaccines":[],"vet":[],"treatments":[{"id":"tr1","label":"Antiparasitaire","every":"3 mois","last":"2026-03-01","next":"2026-06-01"}],"weightLog":[]}`)

	resp := doReq(t, h, http.MethodPost, "/api/pets/p1/treatments/tr1/complete",
		`{"today":"2026-06-26"}`)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("completeTreatment: %d", resp.StatusCode)
	}

	resp = doReq(t, h, http.MethodGet, "/api/pets", "")
	var pets []model.Pet
	decodeJSON(t, resp, &pets)
	if pets[0].Treatments[0].Last != "2026-06-26" {
		t.Fatalf("treatment last not updated: %s", pets[0].Treatments[0].Last)
	}
	if pets[0].Treatments[0].Next != "2026-09-26" {
		t.Fatalf("treatment next not updated: %s", pets[0].Treatments[0].Next)
	}
}
