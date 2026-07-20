package api_test

import (
	"net/http"
	"testing"
	"time"

	"foyer/taskflow/internal/model"
)

// lastMonday renvoie le lundi 00:00 le plus récent (aujourd'hui inclus),
// au format stocké pour Due. Une tâche semaine_libre avec cette Due est
// à la fois alignée sur la frontière de cycle attendue et déjà en retard,
// ce qui permet de simuler un rollover sans que la normalisation de
// createTask ne l'écrase.
func lastMonday(t *testing.T) string {
	t.Helper()
	now := time.Now()
	monday := (int(now.Weekday()) + 6) % 7 // 0=Lun…6=Dim
	d := now.AddDate(0, 0, -monday)
	return time.Date(d.Year(), d.Month(), d.Day(), 0, 0, 0, 0, d.Location()).Format("2006-01-02T15:04")
}

func TestCreateWeeklyFreeTaskNormalizesMisalignedDue(t *testing.T) {
	h := newTestHandler(t)
	misaligned := time.Now().AddDate(0, 0, 3).Format("2006-01-02T15:04")
	doReq(t, h, http.MethodPost, "/api/tasks",
		`{"id":"t1","title":"Passer l'aspirateur","cat":"maison","due":"`+misaligned+`","recurring":true,"repeat":"semaine_libre"}`)

	resp := doReq(t, h, http.MethodGet, "/api/tasks", "")
	var tasks []model.Task
	decodeJSON(t, resp, &tasks)
	if tasks[0].Due == misaligned {
		t.Fatalf("expected the misaligned due %q to be corrected to the next Monday boundary", misaligned)
	}
	due, err := time.Parse("2006-01-02T15:04", tasks[0].Due)
	if err != nil {
		t.Fatalf("parse due: %v", err)
	}
	if due.Weekday() != time.Monday || due.Hour() != 0 || due.Minute() != 0 {
		t.Fatalf("expected due normalized to a Monday at 00:00, got %q", tasks[0].Due)
	}
}

func TestCompleteWeeklyFreeIncrementsCountWithoutMovingDue(t *testing.T) {
	h := newTestHandler(t)
	doReq(t, h, http.MethodPost, "/api/tasks",
		`{"id":"t1","title":"Passer l'aspirateur","cat":"maison","recurring":true,"repeat":"semaine_libre"}`)
	doReq(t, h, http.MethodPost, "/api/members",
		`{"id":"m1","name":"Alice","initial":"A","tone":"rose"}`)

	resp := doReq(t, h, http.MethodGet, "/api/tasks", "")
	var tasks []model.Task
	decodeJSON(t, resp, &tasks)
	due := tasks[0].Due

	doReq(t, h, http.MethodPost, "/api/tasks/t1/complete", `{"memberId":"m1","histId":"h1"}`)

	resp = doReq(t, h, http.MethodGet, "/api/tasks", "")
	decodeJSON(t, resp, &tasks)
	if tasks[0].Due != due {
		t.Fatalf("due should stay put until weekly rollover, got %q (expected %q)", tasks[0].Due, due)
	}
	if tasks[0].WeeklyCount != 1 {
		t.Fatalf("expected weeklyCount=1, got %d", tasks[0].WeeklyCount)
	}
	if !tasks[0].Done {
		t.Fatal("target defaults to 1, task should be done after a single completion")
	}
	if tasks[0].LastDoneAt == nil {
		t.Fatal("expected lastDoneAt to be set")
	}
}

func TestCompleteWeeklyFreeWithTargetRequiresMultipleCompletions(t *testing.T) {
	h := newTestHandler(t)
	doReq(t, h, http.MethodPost, "/api/tasks",
		`{"id":"t1","title":"Aspirateur","cat":"maison","recurring":true,"repeat":"semaine_libre","weeklyTarget":2}`)
	doReq(t, h, http.MethodPost, "/api/members",
		`{"id":"m1","name":"Alice","initial":"A","tone":"rose"}`)

	doReq(t, h, http.MethodPost, "/api/tasks/t1/complete", `{"memberId":"m1","histId":"h1"}`)
	resp := doReq(t, h, http.MethodGet, "/api/tasks", "")
	var tasks []model.Task
	decodeJSON(t, resp, &tasks)
	if tasks[0].Done || tasks[0].WeeklyCount != 1 {
		t.Fatalf("expected weeklyCount=1 and not done after 1st completion, got count=%d done=%v", tasks[0].WeeklyCount, tasks[0].Done)
	}

	doReq(t, h, http.MethodPost, "/api/tasks/t1/complete", `{"memberId":"m1","histId":"h2"}`)
	resp = doReq(t, h, http.MethodGet, "/api/tasks", "")
	decodeJSON(t, resp, &tasks)
	if !tasks[0].Done || tasks[0].WeeklyCount != 2 {
		t.Fatalf("expected weeklyCount=2 and done after 2nd completion, got count=%d done=%v", tasks[0].WeeklyCount, tasks[0].Done)
	}

	// A 3rd completion should not overshoot the target.
	doReq(t, h, http.MethodPost, "/api/tasks/t1/complete", `{"memberId":"m1","histId":"h3"}`)
	resp = doReq(t, h, http.MethodGet, "/api/tasks", "")
	decodeJSON(t, resp, &tasks)
	if tasks[0].WeeklyCount != 2 {
		t.Fatalf("weeklyCount should be capped at target=2, got %d", tasks[0].WeeklyCount)
	}
}

func TestRolloverWeeklyTasksResetsWithoutHistoryEntry(t *testing.T) {
	h := newTestHandler(t)
	past := lastMonday(t)
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
	if tasks[0].WeeklyCount != 0 {
		t.Fatalf("weeklyCount should reset to 0, got %d", tasks[0].WeeklyCount)
	}

	resp = doReq(t, h, http.MethodGet, "/api/history", "")
	var hist []model.HistoryEntry
	decodeJSON(t, resp, &hist)
	if len(hist) != 0 {
		t.Fatalf("rollover should not log any history entry, got %+v", hist)
	}
}

func TestRolloverWeeklyTasksSkipsFullyCompletedAndOtherRepeats(t *testing.T) {
	h := newTestHandler(t)
	past := lastMonday(t)
	doReq(t, h, http.MethodPost, "/api/tasks",
		`{"id":"t1","title":"Déjà faite","cat":"maison","due":"`+past+`","recurring":true,"repeat":"semaine_libre","weeklyCount":1}`)
	doReq(t, h, http.MethodPost, "/api/tasks",
		`{"id":"t2","title":"Sortir poubelles","cat":"maison","due":"`+past+`","recurring":true,"repeat":"semaine","weekDays":[0]}`)

	h.RolloverWeeklyTasks()

	resp := doReq(t, h, http.MethodGet, "/api/history", "")
	var hist []model.HistoryEntry
	decodeJSON(t, resp, &hist)
	if len(hist) != 0 {
		t.Fatalf("expected no history entries, got %+v", hist)
	}

	resp = doReq(t, h, http.MethodGet, "/api/tasks", "")
	var tasks []model.Task
	decodeJSON(t, resp, &tasks)
	for _, task := range tasks {
		if task.ID == "t1" && task.WeeklyCount != 0 {
			t.Fatalf("t1 weeklyCount should still reset to 0 on rollover, got %d", task.WeeklyCount)
		}
	}
}
