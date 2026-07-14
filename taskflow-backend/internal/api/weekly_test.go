package api_test

import (
	"net/http"
	"testing"
	"time"

	"foyer/taskflow/internal/model"
)

func TestCompleteWeeklyFreeAdvancesBySevenDays(t *testing.T) {
	h := newTestHandler(t)
	dueTime := time.Now().AddDate(0, 0, 3)
	due := dueTime.Format("2006-01-02T15:04")
	doReq(t, h, http.MethodPost, "/api/tasks",
		`{"id":"t1","title":"Passer l'aspirateur","cat":"maison","due":"`+due+`","recurring":true,"repeat":"semaine_libre"}`)
	doReq(t, h, http.MethodPost, "/api/members",
		`{"id":"m1","name":"Alice","initial":"A","tone":"rose"}`)

	doReq(t, h, http.MethodPost, "/api/tasks/t1/complete", `{"memberId":"m1","histId":"h1"}`)

	resp := doReq(t, h, http.MethodGet, "/api/tasks", "")
	var tasks []model.Task
	decodeJSON(t, resp, &tasks)
	if tasks[0].Done {
		t.Fatal("weekly-free task should not be marked done (cycle just advances)")
	}
	expected := dueTime.AddDate(0, 0, 7).Format("2006-01-02T15:04")
	if tasks[0].Due != expected {
		t.Fatalf("expected due to advance by 7 days to %q, got %q", expected, tasks[0].Due)
	}
}

func TestRolloverWeeklyTasksLogsMissedAndResets(t *testing.T) {
	h := newTestHandler(t)
	past := time.Now().Add(-2 * time.Hour).Format("2006-01-02T15:04")
	doReq(t, h, http.MethodPost, "/api/tasks",
		`{"id":"t1","title":"Ranger le garage","cat":"maison","due":"`+past+`","recurring":true,"repeat":"semaine_libre"}`)

	h.RolloverWeeklyTasks()

	resp := doReq(t, h, http.MethodGet, "/api/tasks", "")
	var tasks []model.Task
	decodeJSON(t, resp, &tasks)
	if len(tasks) != 1 {
		t.Fatalf("expected 1 task, got %d", len(tasks))
	}
	today := time.Now().Format("2006-01-02")
	if tasks[0].Due[:10] < today {
		t.Fatalf("due date %q should have advanced past today (%s)", tasks[0].Due, today)
	}
	if tasks[0].Done {
		t.Fatal("rolled-over task should not be marked done")
	}

	resp = doReq(t, h, http.MethodGet, "/api/history", "")
	var hist []model.HistoryEntry
	decodeJSON(t, resp, &hist)
	if len(hist) != 1 || hist[0].Action != model.HistActionMissed || hist[0].TaskID != "t1" {
		t.Fatalf("expected one 'missed' history entry for t1, got %+v", hist)
	}
}

func TestRolloverWeeklyTasksSkipsCompletedAndOtherRepeats(t *testing.T) {
	h := newTestHandler(t)
	past := time.Now().Add(-2 * time.Hour).Format("2006-01-02T15:04")
	doReq(t, h, http.MethodPost, "/api/tasks",
		`{"id":"t1","title":"Déjà faite","cat":"maison","due":"`+past+`","recurring":true,"repeat":"semaine_libre","done":true}`)
	doReq(t, h, http.MethodPost, "/api/tasks",
		`{"id":"t2","title":"Sortir poubelles","cat":"maison","due":"`+past+`","recurring":true,"repeat":"semaine","weekDays":[0]}`)

	h.RolloverWeeklyTasks()

	resp := doReq(t, h, http.MethodGet, "/api/history", "")
	var hist []model.HistoryEntry
	decodeJSON(t, resp, &hist)
	if len(hist) != 0 {
		t.Fatalf("expected no history entries, got %+v", hist)
	}
}
