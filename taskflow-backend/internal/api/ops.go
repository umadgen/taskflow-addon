package api

import (
	"encoding/json"
	"net/http"
	"time"

	"foyer/taskflow/internal/model"
)

type opsRequest struct {
	Type     string          `json:"type"`
	ID       string          `json:"id"`
	ItemID   string          `json:"itemId"`
	MemberID string          `json:"memberId"`
	At       string          `json:"at"`
	HistID   string          `json:"histId"`
	NewID    string          `json:"newId"`
	Task     json.RawMessage `json:"task"`
	Patch    json.RawMessage `json:"patch"`
	Member   json.RawMessage `json:"member"`
}

func (h *Handler) handleOps(w http.ResponseWriter, r *http.Request) {
	var op opsRequest
	if err := json.NewDecoder(r.Body).Decode(&op); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	var seq int
	var err error

	switch op.Type {
	case "addTask":
		var t model.Task
		if err = json.Unmarshal(op.Task, &t); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		if op.ID != "" {
			t.ID = op.ID
		}
		if t.ID == "" {
			t.ID = newID()
		}
		if err = h.db.UpsertTask(t); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		seq, _ = h.db.IncrSeq()
		go h.notify(seq)

	case "editTask":
		existing, err2 := h.db.GetTask(op.ID)
		if err2 != nil {
			writeError(w, http.StatusInternalServerError, err2.Error())
			return
		}
		if existing == nil {
			writeError(w, http.StatusNotFound, "task not found")
			return
		}
		if err = json.Unmarshal(op.Patch, existing); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		existing.ID = op.ID
		if err = h.db.UpsertTask(*existing); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		seq, _ = h.db.IncrSeq()
		go h.notify(seq)

	case "deleteTask":
		if err = h.db.DeleteTask(op.ID); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		seq, _ = h.db.IncrSeq()
		go h.notify(seq)

	case "completeTask":
		task, err2 := h.db.GetTask(op.ID)
		if err2 != nil {
			writeError(w, http.StatusInternalServerError, err2.Error())
			return
		}
		if task == nil {
			writeError(w, http.StatusNotFound, "task not found")
			return
		}
		at := op.At
		if at == "" {
			at = time.Now().UTC().Format(time.RFC3339)
		}
		histID := op.HistID
		if histID == "" {
			histID = newID()
		}
		entry := model.HistoryEntry{
			ID:     histID,
			Title:  task.Title,
			Cat:    task.Cat,
			By:     op.MemberID,
			At:     at,
			TaskID: task.ID,
		}
		if err = h.db.InsertHistory(entry); err != nil {
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
			task.DoneBy = &op.MemberID
			task.DoneAt = &at
		}
		if err = h.db.UpsertTask(*task); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		seq, _ = h.db.IncrSeq()
		go h.notify(seq)

	case "addMember":
		var m model.Member
		if err = json.Unmarshal(op.Member, &m); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		if op.ID != "" {
			m.ID = op.ID
		}
		if m.ID == "" {
			m.ID = newID()
		}
		if err = h.db.UpsertMember(m); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		seq, _ = h.db.IncrSeq()
		go h.notify(seq)

	case "editMember":
		members, err2 := h.db.GetMembers()
		if err2 != nil {
			writeError(w, http.StatusInternalServerError, err2.Error())
			return
		}
		var existing *model.Member
		for i := range members {
			if members[i].ID == op.ID {
				existing = &members[i]
				break
			}
		}
		if existing == nil {
			writeError(w, http.StatusNotFound, "member not found")
			return
		}
		if err = json.Unmarshal(op.Patch, existing); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		existing.ID = op.ID
		if err = h.db.UpsertMember(*existing); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		seq, _ = h.db.IncrSeq()
		go h.notify(seq)

	case "deleteMember":
		if err = h.db.DeleteMember(op.ID); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		seq, _ = h.db.IncrSeq()
		go h.notify(seq)

	case "toggleChecklistItem":
		task, err2 := h.db.GetTask(op.ID)
		if err2 != nil {
			writeError(w, http.StatusInternalServerError, err2.Error())
			return
		}
		if task == nil {
			writeError(w, http.StatusNotFound, "task not found")
			return
		}
		found := false
		for i := range task.Checklist {
			if task.Checklist[i].ID == op.ItemID {
				task.Checklist[i].Done = !task.Checklist[i].Done
				found = true
				break
			}
		}
		if !found {
			writeError(w, http.StatusNotFound, "checklist item not found")
			return
		}
		if err = h.db.UpsertTask(*task); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		seq, _ = h.db.IncrSeq()
		go h.notify(seq)

	default:
		writeError(w, http.StatusBadRequest, "unknown op type: "+op.Type)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"seq": seq, "ok": true})
}
