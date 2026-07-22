package api_test

import (
	"net/http"
	"testing"
	"time"

	"foyer/taskflow/internal/model"
)

func TestRolloverOverdueRecurringTasksLogsMissedAndAdvances(t *testing.T) {
	h := newTestHandler(t)
	yesterday := time.Now().AddDate(0, 0, -1).Format("2006-01-02T15:04")
	doReq(t, h, http.MethodPost, "/api/tasks",
		`{"id":"t1","title":"Sortir le chien","cat":"maison","due":"`+yesterday+`","recurring":true,"repeat":"jour"}`)

	h.RolloverOverdueRecurringTasks()

	resp := doReq(t, h, http.MethodGet, "/api/history", "")
	var hist []model.HistoryEntry
	decodeJSON(t, resp, &hist)
	if len(hist) != 1 || hist[0].Action != model.HistActionMissed || hist[0].TaskID != "t1" {
		t.Fatalf("expected one 'missed' history entry for t1, got %+v", hist)
	}
	if hist[0].By != "" {
		t.Fatalf("missed entry should have no attributed member, got %q", hist[0].By)
	}

	resp = doReq(t, h, http.MethodGet, "/api/tasks", "")
	var tasks []model.Task
	decodeJSON(t, resp, &tasks)
	today := time.Now().Format("2006-01-02")
	if tasks[0].Due[:10] < today {
		t.Fatalf("due date %q should have advanced to today or later", tasks[0].Due)
	}
	if tasks[0].Done {
		t.Fatal("rolled-over task should not be marked done")
	}
}

func TestRolloverOverdueRecurringTasksSkipsWeeklyFreeDoneAndFuture(t *testing.T) {
	h := newTestHandler(t)
	yesterday := time.Now().AddDate(0, 0, -1).Format("2006-01-02T15:04")
	tomorrow := time.Now().AddDate(0, 0, 1).Format("2006-01-02T15:04")

	doReq(t, h, http.MethodPost, "/api/tasks",
		`{"id":"free","title":"Libre service","cat":"maison","due":"`+yesterday+`","recurring":true,"repeat":"semaine_libre"}`)
	doReq(t, h, http.MethodPost, "/api/tasks",
		`{"id":"done","title":"Déjà faite","cat":"maison","due":"`+yesterday+`","recurring":true,"repeat":"jour","done":true}`)
	doReq(t, h, http.MethodPost, "/api/tasks",
		`{"id":"future","title":"Pas encore due","cat":"maison","due":"`+tomorrow+`","recurring":true,"repeat":"jour"}`)

	h.RolloverOverdueRecurringTasks()

	resp := doReq(t, h, http.MethodGet, "/api/history", "")
	var hist []model.HistoryEntry
	decodeJSON(t, resp, &hist)
	if len(hist) != 0 {
		t.Fatalf("expected no history entries, got %+v", hist)
	}
}

func TestRolloverOverdueRecurringTasksSkipsDuringVacation(t *testing.T) {
	h := newTestHandler(t)
	yesterday := time.Now().AddDate(0, 0, -1).Format("2006-01-02T15:04")
	until := time.Now().AddDate(0, 0, 5).Format("2006-01-02")

	doReq(t, h, http.MethodPatch, "/api/settings",
		`{"vacationMode":true,"vacationUntil":"`+until+`"}`)
	doReq(t, h, http.MethodPost, "/api/tasks",
		`{"id":"t1","title":"Sortir le chien","cat":"maison","due":"`+yesterday+`","recurring":true,"repeat":"jour"}`)

	h.RolloverOverdueRecurringTasks()

	resp := doReq(t, h, http.MethodGet, "/api/history", "")
	var hist []model.HistoryEntry
	decodeJSON(t, resp, &hist)
	if len(hist) != 0 {
		t.Fatalf("expected no history entries while on vacation, got %+v", hist)
	}

	resp = doReq(t, h, http.MethodGet, "/api/tasks", "")
	var tasks []model.Task
	decodeJSON(t, resp, &tasks)
	if tasks[0].Due != yesterday {
		t.Fatalf("due date should stay put during vacation, got %q (expected %q)", tasks[0].Due, yesterday)
	}
}
