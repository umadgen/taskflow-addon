package api_test

import (
	"net/http"
	"testing"
	"time"

	"foyer/taskflow/internal/model"
)

func TestTasksLifecycle(t *testing.T) {
	h := newTestHandler(t)

	// Créer
	resp := doReq(t, h, http.MethodPost, "/api/tasks",
		`{"id":"t1","title":"Vaisselle","cat":"maison","due":"2026-06-27T18:00","recurring":false}`)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("create: %d", resp.StatusCode)
	}

	// Lister
	resp = doReq(t, h, http.MethodGet, "/api/tasks", "")
	var tasks []model.Task
	decodeJSON(t, resp, &tasks)
	if len(tasks) != 1 || tasks[0].Title != "Vaisselle" {
		t.Fatalf("list: %+v", tasks)
	}

	// Modifier
	resp = doReq(t, h, http.MethodPatch, "/api/tasks/t1", `{"title":"Grande vaisselle"}`)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("patch: %d", resp.StatusCode)
	}

	// Supprimer
	resp = doReq(t, h, http.MethodDelete, "/api/tasks/t1", "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("delete: %d", resp.StatusCode)
	}
}

func TestCompleteNonRecurring(t *testing.T) {
	h := newTestHandler(t)
	doReq(t, h, http.MethodPost, "/api/tasks",
		`{"id":"t1","title":"Poubelles","cat":"maison","due":"2026-06-27T08:00","recurring":false}`)
	doReq(t, h, http.MethodPost, "/api/members",
		`{"id":"m1","name":"Alice","initial":"A","tone":"rose"}`)

	resp := doReq(t, h, http.MethodPost, "/api/tasks/t1/complete",
		`{"memberId":"m1","histId":"h1","at":"2026-06-27T09:00:00Z"}`)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("complete: %d", resp.StatusCode)
	}

	// La tâche doit être marquée done
	resp = doReq(t, h, http.MethodGet, "/api/tasks", "")
	var tasks []model.Task
	decodeJSON(t, resp, &tasks)
	if !tasks[0].Done {
		t.Fatal("expected done=true after complete")
	}

	// Un entrée d'historique doit exister
	resp = doReq(t, h, http.MethodGet, "/api/history", "")
	var hist []model.HistoryEntry
	decodeJSON(t, resp, &hist)
	if len(hist) != 1 || hist[0].By != "m1" {
		t.Fatalf("history: %+v", hist)
	}
}

func TestCompleteRecurring(t *testing.T) {
	h := newTestHandler(t)
	doReq(t, h, http.MethodPost, "/api/tasks",
		`{"id":"t1","title":"Sortir poubelles","cat":"maison","due":"2026-06-29T08:00","recurring":true,"repeat":"semaine","weekDays":[0]}`)
	doReq(t, h, http.MethodPost, "/api/members",
		`{"id":"m1","name":"Alice","initial":"A","tone":"rose"}`)

	doReq(t, h, http.MethodPost, "/api/tasks/t1/complete",
		`{"memberId":"m1","histId":"h1","at":"2026-06-29T09:00:00Z"}`)

	// La tâche NE doit PAS être done — due avancé à la semaine suivante
	resp := doReq(t, h, http.MethodGet, "/api/tasks", "")
	var tasks []model.Task
	decodeJSON(t, resp, &tasks)
	if tasks[0].Done {
		t.Fatal("recurring task should not be marked done")
	}
	if tasks[0].Due == "2026-06-29T08:00" {
		t.Fatal("due date should have advanced")
	}
}

func TestCompleteVeryOverdueRecurringClearsLate(t *testing.T) {
	h := newTestHandler(t)
	// Tâche hebdomadaire en retard depuis des années, marquée "late".
	doReq(t, h, http.MethodPost, "/api/tasks",
		`{"id":"t1","title":"Sortir poubelles","cat":"maison","due":"2020-01-06T08:00","recurring":true,"repeat":"semaine","weekDays":[0],"late":true}`)
	doReq(t, h, http.MethodPost, "/api/members",
		`{"id":"m1","name":"Alice","initial":"A","tone":"rose"}`)

	doReq(t, h, http.MethodPost, "/api/tasks/t1/complete",
		`{"memberId":"m1","histId":"h1"}`)

	resp := doReq(t, h, http.MethodGet, "/api/tasks", "")
	var tasks []model.Task
	decodeJSON(t, resp, &tasks)

	if tasks[0].Late {
		t.Fatal("late flag should be cleared after completing the task")
	}
	today := time.Now().Format("2006-01-02")
	if tasks[0].Due[:10] < today {
		t.Fatalf("next due date %q should not still be in the past (today=%s)", tasks[0].Due, today)
	}
}

func TestUncompleteTask(t *testing.T) {
	h := newTestHandler(t)
	doReq(t, h, http.MethodPost, "/api/tasks",
		`{"id":"t1","title":"Courses","cat":"courses","due":"2026-06-27T10:00","recurring":false}`)
	doReq(t, h, http.MethodPost, "/api/members",
		`{"id":"m1","name":"Alice","initial":"A","tone":"rose"}`)
	doReq(t, h, http.MethodPost, "/api/tasks/t1/complete",
		`{"memberId":"m1","histId":"h1","at":"2026-06-27T11:00:00Z"}`)

	resp := doReq(t, h, http.MethodPost, "/api/tasks/t1/uncomplete", "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("uncomplete: %d", resp.StatusCode)
	}
	resp = doReq(t, h, http.MethodGet, "/api/tasks", "")
	var tasks []model.Task
	decodeJSON(t, resp, &tasks)
	if tasks[0].Done {
		t.Fatal("expected done=false after uncomplete")
	}
}
