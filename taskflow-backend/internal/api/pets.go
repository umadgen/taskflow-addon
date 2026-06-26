package api

import (
	"encoding/json"
	"net/http"
	"time"

	chi "github.com/go-chi/chi/v5"

	"foyer/taskflow/internal/model"
)

func (h *Handler) listPets(w http.ResponseWriter, r *http.Request) {
	pets, err := h.db.GetPets()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, pets)
}

func (h *Handler) createPet(w http.ResponseWriter, r *http.Request) {
	var p model.Pet
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if p.ID == "" {
		p.ID = newID()
	}
	if err := h.db.UpsertPet(p); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	seq, _ := h.db.IncrSeq()
	go h.notify(seq)
	writeJSON(w, http.StatusOK, map[string]any{"seq": seq, "pet": p})
}

func (h *Handler) updatePet(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	existing, err := h.db.GetPet(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if existing == nil {
		writeError(w, http.StatusNotFound, "pet not found")
		return
	}
	if err := json.NewDecoder(r.Body).Decode(existing); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	existing.ID = id
	if err := h.db.UpsertPet(*existing); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	seq, _ := h.db.IncrSeq()
	go h.notify(seq)
	writeJSON(w, http.StatusOK, map[string]any{"seq": seq})
}

func (h *Handler) deletePet(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.db.DeletePet(id); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	seq, _ := h.db.IncrSeq()
	go h.notify(seq)
	writeJSON(w, http.StatusOK, map[string]any{"seq": seq})
}

func (h *Handler) completeTreatment(w http.ResponseWriter, r *http.Request) {
	petID := chi.URLParam(r, "id")
	tid := chi.URLParam(r, "tid")

	var body struct {
		Today string `json:"today"`
	}
	json.NewDecoder(r.Body).Decode(&body)
	today := body.Today
	if today == "" {
		today = time.Now().Format("2006-01-02")
	}

	pet, err := h.db.GetPet(petID)
	if err != nil || pet == nil {
		writeError(w, http.StatusNotFound, "pet not found")
		return
	}
	for i := range pet.Treatments {
		if pet.Treatments[i].ID == tid {
			from, _ := time.Parse("2006-01-02", today)
			next := addEvery(from, pet.Treatments[i].Every)
			pet.Treatments[i].Last = today
			pet.Treatments[i].Next = next.Format("2006-01-02")
			break
		}
	}
	if err := h.db.UpsertPet(*pet); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	seq, _ := h.db.IncrSeq()
	go h.notify(seq)
	writeJSON(w, http.StatusOK, map[string]any{"seq": seq})
}

func (h *Handler) completeVaccine(w http.ResponseWriter, r *http.Request) {
	petID := chi.URLParam(r, "id")
	vid := chi.URLParam(r, "vid")

	var body struct {
		Today string `json:"today"`
		Next  string `json:"next"`
	}
	json.NewDecoder(r.Body).Decode(&body)
	today := body.Today
	if today == "" {
		today = time.Now().Format("2006-01-02")
	}

	pet, err := h.db.GetPet(petID)
	if err != nil || pet == nil {
		writeError(w, http.StatusNotFound, "pet not found")
		return
	}
	for i := range pet.Vaccines {
		if pet.Vaccines[i].ID == vid {
			pet.Vaccines[i].Date = today
			pet.Vaccines[i].Done = true
			if body.Next != "" {
				pet.Vaccines[i].Next = body.Next
			}
			break
		}
	}
	if err := h.db.UpsertPet(*pet); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	seq, _ := h.db.IncrSeq()
	go h.notify(seq)
	writeJSON(w, http.StatusOK, map[string]any{"seq": seq})
}
