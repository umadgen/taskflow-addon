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
	if isWeeklyFree(t) && !isWeeklyFreeAligned(t.Due) {
		t.Due = weeklyFreeDue(time.Now())
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
	if isWeeklyFree(*existing) && !isWeeklyFreeAligned(existing.Due) {
		existing.Due = weeklyFreeDue(time.Now())
	}
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
		Action: model.HistActionCompleted,
	}
	if err := h.db.InsertHistory(entry); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if task.Recurring {
		if isWeeklyFree(*task) {
			applyWeeklyFreeCompletion(task, body.MemberID, at)
		} else {
			task.Due = advanceDue(*task)
			task.Done = false
			task.DoneBy = nil
			task.DoneAt = nil
		}
	} else {
		task.Done = true
		task.DoneBy = &body.MemberID
		task.DoneAt = &at
	}
	task.Late = false

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

// isWeeklyFree indique si t est une tâche hebdomadaire "libre service"
// (model.RepeatWeeklyFree) : une ou plusieurs fois par semaine, n'importe
// quel jour, plutôt qu'un jour fixe.
func isWeeklyFree(t model.Task) bool {
	return t.Repeat != nil && *t.Repeat == model.RepeatWeeklyFree
}

// weeklyTarget renvoie le nombre de fois par semaine requis pour t (1 par défaut).
func weeklyTarget(t model.Task) int {
	if t.WeeklyTarget != nil && *t.WeeklyTarget > 0 {
		return *t.WeeklyTarget
	}
	return 1
}

// weeklyFreeDue calcule la frontière de cycle (prochain lundi 00:00) d'une
// tâche hebdomadaire "libre service" à partir de now. Utilisée pour corriger
// côté serveur toute Due mal alignée (formulaire, import en masse, ancienne
// donnée) afin que toutes les tâches semaine_libre partagent le même repère
// hebdomadaire. Reprend exactement la logique de nextWeekBoundary() côté
// admin.html.
func weeklyFreeDue(now time.Time) string {
	const layout = "2006-01-02T15:04"
	goWD := int(now.Weekday()) // 0=Dim…6=Sam
	tfWD := (goWD + 6) % 7     // 0=Lun…6=Dim
	midnight := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	return midnight.AddDate(0, 0, 7-tfWD).Format(layout)
}

// isWeeklyFreeAligned indique si due tombe exactement sur la frontière de
// cycle attendue (un lundi 00:00), au format stocké par weeklyFreeDue/le
// formulaire admin.
func isWeeklyFreeAligned(due string) bool {
	t, err := time.Parse("2006-01-02T15:04", due)
	if err != nil {
		return false
	}
	return t.Weekday() == time.Monday && t.Hour() == 0 && t.Minute() == 0
}

// applyWeeklyFreeCompletion incrémente le compteur hebdomadaire d'une tâche
// "libre service" sans avancer son échéance : le cycle (et le compteur) ne
// sont réinitialisés qu'au passage de semaine par RolloverWeeklyTasks.
func applyWeeklyFreeCompletion(task *model.Task, memberID, at string) {
	target := weeklyTarget(*task)
	if task.WeeklyCount < target {
		task.WeeklyCount++
	}
	task.LastDoneAt = &at
	task.Done = task.WeeklyCount >= target
	if task.Done {
		mID := memberID
		task.DoneBy = &mID
		task.DoneAt = &at
	} else {
		task.DoneBy = nil
		task.DoneAt = nil
	}
}

// advanceDue calcule la prochaine échéance d'une tâche récurrente.
// Weekdays Taskflow : 0=Lun…6=Dim. Go Weekday : 0=Dim…6=Sam.
// Si la tâche était en retard depuis plus d'un cycle, on avance jusqu'à
// obtenir une échéance qui n'est plus dans le passé, pour éviter qu'elle
// ne réapparaisse aussitôt comme en retard.
func advanceDue(t model.Task) string {
	const layout = "2006-01-02T15:04"
	due, err := time.Parse(layout, t.Due)
	if err != nil {
		due, _ = time.Parse(time.RFC3339, t.Due)
	}

	today := time.Now().Format("2006-01-02")
	for {
		due = advanceDueOnce(t, due)
		if due.Format("2006-01-02") >= today {
			break
		}
	}

	return due.Format(layout)
}

// advanceDueOnce calcule une seule occurrence suivante à partir de due.
func advanceDueOnce(t model.Task, due time.Time) time.Time {
	repeat := ""
	if t.Repeat != nil {
		repeat = *t.Repeat
	}

	switch repeat {
	case "jour":
		return due.AddDate(0, 0, 1)

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
		return due.AddDate(0, 0, next)

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
		return time.Date(y, m, md, due.Hour(), due.Minute(), 0, 0, due.Location())

	case model.RepeatWeeklyFree:
		// Due marque toujours le lundi qui clôt le cycle en cours ; le cycle
		// suivant se termine donc simplement une semaine plus tard.
		return due.AddDate(0, 0, 7)

	default:
		return due.AddDate(0, 0, 7)
	}
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
