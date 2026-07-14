package api

import (
	"log"
	"time"

	"foyer/taskflow/internal/model"
)

// RolloverWeeklyTasks clôture les tâches hebdomadaires "libre service"
// (model.RepeatWeeklyFree) dont le cycle est terminé (Due dépassé) sans
// avoir été cochées : une entrée d'historique "missed" est tracée, puis la
// tâche est réinitialisée pour la semaine suivante. Appelée périodiquement
// depuis main.go, faute de scheduler côté navigateur pour ces tâches.
func (h *Handler) RolloverWeeklyTasks() {
	tasks, err := h.db.GetTasks()
	if err != nil {
		log.Printf("weekly-rollover: GetTasks: %v", err)
		return
	}

	now := time.Now().Format("2006-01-02T15:04")
	changed := false
	for _, t := range tasks {
		if !t.Recurring || t.Repeat == nil || *t.Repeat != model.RepeatWeeklyFree || t.Done {
			continue
		}
		if t.Due > now {
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
			log.Printf("weekly-rollover: InsertHistory %s: %v", t.ID, err)
			continue
		}

		t.Due = advanceDue(t)
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
