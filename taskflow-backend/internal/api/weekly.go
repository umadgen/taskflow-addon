package api

import (
	"log"
	"time"
)

// RolloverWeeklyTasks clôture les tâches hebdomadaires "libre service"
// (model.RepeatWeeklyFree) dont le cycle est terminé (Due dépassé), qu'elles
// aient été cochées ou non, et les réinitialise pour la semaine suivante.
// Appelée périodiquement depuis main.go, faute de scheduler côté navigateur
// pour ces tâches.
func (h *Handler) RolloverWeeklyTasks() {
	tasks, err := h.db.GetTasks()
	if err != nil {
		log.Printf("weekly-rollover: GetTasks: %v", err)
		return
	}

	now := time.Now().Format("2006-01-02T15:04")
	changed := false
	for _, t := range tasks {
		if !isWeeklyFree(t) {
			continue
		}
		if t.Due > now {
			continue
		}

		t.Due = advanceDue(t)
		t.WeeklyCount = 0
		t.Done = false
		t.DoneBy = nil
		t.DoneAt = nil
		t.Late = false
		if err := h.db.UpsertTask(t); err != nil {
			log.Printf("weekly-rollover: UpsertTask %s: %v", t.ID, err)
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
