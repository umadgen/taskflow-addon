package api

import (
	"log"
	"time"

	"foyer/taskflow/internal/model"
)

// RolloverOverdueRecurringTasks clôture les tâches récurrentes à échéance
// fixe (quotidienne, hebdomadaire à jour(s) fixe(s), mensuelle — pas les
// "semaine_libre", gérées séparément par RolloverWeeklyTasks) dont la Due
// est dépassée sans avoir été cochées : une entrée d'historique "missed" est
// tracée, puis l'échéance avance normalement au prochain cycle. Sans ça, la
// tâche restait "en retard" indéfiniment et il fallait la cocher pour la
// faire avancer — ce qui, pour une tâche quotidienne, la faisait réapparaître
// aussitôt due "aujourd'hui" : il fallait donc la faire deux fois. Appelée
// périodiquement depuis main.go, faute de scheduler côté navigateur.
func (h *Handler) RolloverOverdueRecurringTasks() {
	tasks, err := h.db.GetTasks()
	if err != nil {
		log.Printf("overdue-rollover: GetTasks: %v", err)
		return
	}

	today := time.Now().Format("2006-01-02")
	settings, _ := h.db.GetSettings()
	onVacation := settings.VacationMode && settings.VacationUntil >= today

	changed := false
	for _, t := range tasks {
		if !t.Recurring || t.Done || t.Due == "" || isWeeklyFree(t) {
			continue
		}
		if t.Due[:10] >= today {
			continue
		}
		// Vacation mode exists precisely so missed occurrences during the
		// break don't pile up; let completeTask's own vacation skip-ahead
		// (see ops.go) catch these up once vacation ends.
		if onVacation && t.Due[:10] <= settings.VacationUntil {
			continue
		}

		entry := model.HistoryEntry{
			ID:     newID(),
			Title:  t.Title,
			Cat:    t.Cat,
			By:     "",
			At:     t.Due,
			TaskID: t.ID,
			Action: model.HistActionMissed,
		}
		if err := h.db.InsertHistory(entry); err != nil {
			log.Printf("overdue-rollover: InsertHistory %s: %v", t.ID, err)
			continue
		}

		t.Due = advanceDue(t)
		t.Done = false
		t.DoneBy = nil
		t.DoneAt = nil
		t.Late = false
		if err := h.db.UpsertTask(t); err != nil {
			log.Printf("overdue-rollover: UpsertTask %s: %v", t.ID, err)
			continue
		}
		changed = true
	}

	if changed {
		if seq, err := h.db.IncrSeq(); err == nil {
			h.notify(seq)
		}
	}
}
