package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"foyer/taskflow/internal/model"
)

func (h *Handler) exportData(w http.ResponseWriter, r *http.Request) {
	tasks, _    := h.db.GetTasks()
	members, _  := h.db.GetMembers()
	history, _  := h.db.GetHistory()
	pets, _     := h.db.GetPets()
	settings, _ := h.db.GetSettings()

	data := model.AppData{
		Version:      1,
		Members:      members,
		Tasks:        tasks,
		History:      history,
		Pets:         pets,
		Settings:     settings,
		GoogleEvents: []model.GoogleEvent{},
	}

	filename := fmt.Sprintf("taskflow-backup-%s.json", time.Now().Format("2006-01-02"))
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Disposition", `attachment; filename="`+filename+`"`)
	json.NewEncoder(w).Encode(data)
}

func (h *Handler) restoreData(w http.ResponseWriter, r *http.Request) {
	var data model.AppData
	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		writeError(w, http.StatusBadRequest, "JSON invalide : "+err.Error())
		return
	}
	if err := h.db.ClearAll(); err != nil {
		writeError(w, http.StatusInternalServerError, "effacement : "+err.Error())
		return
	}
	if err := h.db.ImportData(data); err != nil {
		writeError(w, http.StatusInternalServerError, "import : "+err.Error())
		return
	}
	seq, _ := h.db.IncrSeq()
	go h.notify(seq)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "seq": seq})
}
