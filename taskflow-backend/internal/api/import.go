package api

import (
	"encoding/json"
	"net/http"

	"foyer/taskflow/internal/model"
)

func (h *Handler) importData(w http.ResponseWriter, r *http.Request) {
	if h.secret == "" || r.Header.Get("x-foyer-secret") != h.secret {
		writeError(w, http.StatusForbidden, "secret invalide ou manquant")
		return
	}
	empty, err := h.db.IsEmpty()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if !empty {
		writeError(w, http.StatusConflict, "la base de données n'est pas vide")
		return
	}
	var data model.AppData
	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := h.db.ImportData(data); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	seq, _ := h.db.IncrSeq()
	go h.notify(seq)
	writeJSON(w, http.StatusOK, map[string]any{"seq": seq, "imported": true})
}
