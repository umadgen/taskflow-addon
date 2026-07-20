package api

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"foyer/taskflow/internal/model"
)

// bulkTaskInput est le format attendu pour une tâche dans un import en masse.
// L'assignee est désigné par nom (résolu vers l'ID membre) plutôt que par ID technique.
type bulkTaskInput struct {
	Title     string                 `json:"title"`
	Cat       string                 `json:"cat"`
	Assignee  string                 `json:"assignee"`
	Due       string                 `json:"due"`
	Recurring bool                   `json:"recurring"`
	Repeat    *string                `json:"repeat"`
	WeekDays  []int                  `json:"weekDays"`
	MonthDay  *int                   `json:"monthDay"`
	Time      *string                `json:"time"`
	FreqText  *string                `json:"freqText"`
	Checklist []model.ChecklistItem  `json:"checklist"`
}

type bulkRequest struct {
	Tasks []bulkTaskInput `json:"tasks"`
}

type bulkError struct {
	Index int    `json:"index"`
	Title string `json:"title"`
	Error string `json:"error"`
}

type bulkResponse struct {
	Seq      int         `json:"seq"`
	Created  int         `json:"created"`
	Warnings []bulkError `json:"warnings"`
	Errors   []bulkError `json:"errors"`
}

func (h *Handler) bulkCreateTasks(w http.ResponseWriter, r *http.Request) {
	var req bulkRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	members, err := h.db.GetMembers()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	memberByName := make(map[string]string, len(members))
	for _, m := range members {
		memberByName[strings.ToLower(strings.TrimSpace(m.Name))] = m.ID
	}

	resp := bulkResponse{Warnings: []bulkError{}, Errors: []bulkError{}}

	for i, in := range req.Tasks {
		title := strings.TrimSpace(in.Title)
		if title == "" {
			resp.Errors = append(resp.Errors, bulkError{Index: i, Title: in.Title, Error: "titre manquant"})
			continue
		}

		for j := range in.Checklist {
			if in.Checklist[j].ID == "" {
				in.Checklist[j].ID = newID()
			}
		}

		task := model.Task{
			ID:        newID(),
			Title:     title,
			Cat:       in.Cat,
			Due:       in.Due,
			Recurring: in.Recurring,
			Repeat:    in.Repeat,
			WeekDays:  in.WeekDays,
			MonthDay:  in.MonthDay,
			Time:      in.Time,
			FreqText:  in.FreqText,
			Checklist: in.Checklist,
		}
		if isWeeklyFree(task) && !isWeeklyFreeAligned(task.Due) {
			task.Due = weeklyFreeDue(time.Now())
		}

		if name := strings.ToLower(strings.TrimSpace(in.Assignee)); name != "" {
			if id, ok := memberByName[name]; ok {
				task.Assignee = &id
			} else {
				resp.Warnings = append(resp.Warnings, bulkError{Index: i, Title: title, Error: "membre '" + in.Assignee + "' introuvable, tâche créée sans assigné"})
			}
		}

		if err := h.db.UpsertTask(task); err != nil {
			resp.Errors = append(resp.Errors, bulkError{Index: i, Title: title, Error: err.Error()})
			continue
		}
		resp.Created++
	}

	seq, _ := h.db.IncrSeq()
	go h.notify(seq)
	resp.Seq = seq

	writeJSON(w, http.StatusOK, resp)
}
