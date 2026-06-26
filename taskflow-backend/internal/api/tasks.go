package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	chi "github.com/go-chi/chi/v5"

	"foyer/taskflow/internal/model"
)

func (h *Handler) listTasks(w http.ResponseWriter, r *http.Request) {
	tasks, err := h.db.GetTasks()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, tasks)
}

func (h *Handler) createTask(w http.ResponseWriter, r *http.Request) {
	var t model.Task
	if err := json.NewDecoder(r.Body).Decode(&t); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if t.ID == "" {
		t.ID = newID()
	}
	if err := h.db.UpsertTask(t); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	seq, _ := h.db.IncrSeq()
	go h.notify(seq)
	writeJSON(w, http.StatusOK, map[string]any{"seq": seq, "task": t})
}

func (h *Handler) updateTask(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	existing, err := h.db.GetTask(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if existing == nil {
		writeError(w, http.StatusNotFound, "task not found")
		return
	}
	if err := json.NewDecoder(r.Body).Decode(existing); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	existing.ID = id
	if err := h.db.UpsertTask(*existing); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	seq, _ := h.db.IncrSeq()
	go h.notify(seq)
	writeJSON(w, http.StatusOK, map[string]any{"seq": seq})
}

func (h *Handler) deleteTask(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.db.DeleteTask(id); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	seq, _ := h.db.IncrSeq()
	go h.notify(seq)
	writeJSON(w, http.StatusOK, map[string]any{"seq": seq})
}

func (h *Handler) completeTask(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var body struct {
		MemberID string `json:"memberId"`
		At       string `json:"at"`
		HistID   string `json:"histId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	task, err := h.db.GetTask(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if task == nil {
		writeError(w, http.StatusNotFound, "task not found")
		return
	}
	at := body.At
	if at == "" {
		at = time.Now().UTC().Format(time.RFC3339)
	}
	histID := body.HistID
	if histID == "" {
		histID = newID()
	}

	entry := model.HistoryEntry{
		ID:     histID,
		Title:  task.Title,
		Cat:    task.Cat,
		By:     body.MemberID,
		At:     at,
		TaskID: task.ID,
	}
	if err := h.db.InsertHistory(entry); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if task.Recurring {
		task.Due = advanceDue(*task)
		task.Done = false
		task.DoneBy = nil
		task.DoneAt = nil
	} else {
		task.Done = true
		task.DoneBy = &body.MemberID
		task.DoneAt = &at
	}

	if err := h.db.UpsertTask(*task); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	seq, _ := h.db.IncrSeq()
	go h.notify(seq)
	writeJSON(w, http.StatusOK, map[string]any{"seq": seq})
}

func (h *Handler) uncompleteTask(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	task, err := h.db.GetTask(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if task == nil {
		writeError(w, http.StatusNotFound, "task not found")
		return
	}
	task.Done = false
	task.DoneBy = nil
	task.DoneAt = nil
	if err := h.db.UpsertTask(*task); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	seq, _ := h.db.IncrSeq()
	go h.notify(seq)
	writeJSON(w, http.StatusOK, map[string]any{"seq": seq})
}

// advanceDue calcule la prochaine échéance d'une tâche récurrente.
// Weekdays Taskflow : 0=Lun…6=Dim. Go Weekday : 0=Dim…6=Sam.
func advanceDue(t model.Task) string {
	const layout = "2006-01-02T15:04"
	due, err := time.Parse(layout, t.Due)
	if err != nil {
		due, _ = time.Parse(time.RFC3339, t.Due)
	}

	repeat := ""
	if t.Repeat != nil {
		repeat = *t.Repeat
	}

	switch repeat {
	case "jour":
		due = due.AddDate(0, 0, 1)

	case "semaine":
		goWD := int(due.Weekday()) // 0=Sun…6=Sat
		tfWD := (goWD + 6) % 7    // 0=Mon…6=Sun
		next := 8
		for delta := 1; delta <= 7; delta++ {
			candidate := (tfWD + delta) % 7
			for _, d := range t.WeekDays {
				if d == candidate && delta < next {
					next = delta
				}
			}
		}
		if next == 8 {
			next = 7
		}
		due = due.AddDate(0, 0, next)

	case "mois":
		md := 1
		if t.MonthDay != nil {
			md = *t.MonthDay
		}
		y, m, _ := due.Date()
		m++
		if m > 12 {
			m = 1
			y++
		}
		due = time.Date(y, m, md, due.Hour(), due.Minute(), 0, 0, due.Location())

	default:
		due = due.AddDate(0, 0, 7)
	}

	return due.Format(layout)
}

// addEvery ajoute une durée décrite en texte ("3 mois", "15 jours", "1 an").
func addEvery(from time.Time, every string) time.Time {
	parts := strings.Fields(strings.ToLower(every))
	if len(parts) != 2 {
		return from.AddDate(0, 1, 0)
	}
	n, err := strconv.Atoi(parts[0])
	if err != nil || n <= 0 {
		return from.AddDate(0, 1, 0)
	}
	switch {
	case strings.HasPrefix(parts[1], "jour"):
		return from.AddDate(0, 0, n)
	case strings.HasPrefix(parts[1], "semaine"):
		return from.AddDate(0, 0, n*7)
	case strings.HasPrefix(parts[1], "mois"):
		return from.AddDate(0, n, 0)
	case strings.HasPrefix(parts[1], "an"):
		return from.AddDate(n, 0, 0)
	default:
		return from.AddDate(0, 1, 0)
	}
}
