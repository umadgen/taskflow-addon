package api

import (
	"encoding/json"
	"net/http"

	chi "github.com/go-chi/chi/v5"

	"foyer/taskflow/internal/model"
)

func (h *Handler) listMembers(w http.ResponseWriter, r *http.Request) {
	members, err := h.db.GetMembers()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, members)
}

func (h *Handler) createMember(w http.ResponseWriter, r *http.Request) {
	var m model.Member
	if err := json.NewDecoder(r.Body).Decode(&m); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if m.ID == "" {
		m.ID = newID()
	}
	if err := h.db.UpsertMember(m); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	seq, _ := h.db.IncrSeq()
	go h.notify(seq)
	writeJSON(w, http.StatusOK, map[string]any{"seq": seq, "member": m})
}

func (h *Handler) updateMember(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	members, err := h.db.GetMembers()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	var existing *model.Member
	for i := range members {
		if members[i].ID == id {
			existing = &members[i]
			break
		}
	}
	if existing == nil {
		writeError(w, http.StatusNotFound, "member not found")
		return
	}
	if err := json.NewDecoder(r.Body).Decode(existing); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	existing.ID = id
	if err := h.db.UpsertMember(*existing); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	seq, _ := h.db.IncrSeq()
	go h.notify(seq)
	writeJSON(w, http.StatusOK, map[string]any{"seq": seq})
}

func (h *Handler) deleteMember(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.db.DeleteMember(id); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	seq, _ := h.db.IncrSeq()
	go h.notify(seq)
	writeJSON(w, http.StatusOK, map[string]any{"seq": seq})
}
