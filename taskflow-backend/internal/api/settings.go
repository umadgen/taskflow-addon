package api

import (
	"encoding/json"
	"net/http"
)

func (h *Handler) getSettings(w http.ResponseWriter, r *http.Request) {
	s, err := h.db.GetSettings()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, s)
}

func (h *Handler) updateSettings(w http.ResponseWriter, r *http.Request) {
	existing, err := h.db.GetSettings()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if err := json.NewDecoder(r.Body).Decode(&existing); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := h.db.SaveSettings(existing); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	seq, _ := h.db.IncrSeq()
	go h.notify(seq)
	writeJSON(w, http.StatusOK, map[string]any{"seq": seq})
}
