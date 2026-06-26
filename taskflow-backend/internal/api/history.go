package api

import "net/http"

func (h *Handler) listHistory(w http.ResponseWriter, r *http.Request) {
	history, err := h.db.GetHistory()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, history)
}
