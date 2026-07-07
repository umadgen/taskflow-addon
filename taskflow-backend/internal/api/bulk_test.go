package api_test

import (
	"net/http"
	"testing"

	"foyer/taskflow/internal/model"
)

func TestBulkCreateTasks(t *testing.T) {
	h := newTestHandler(t)
	doReq(t, h, http.MethodPost, "/api/members",
		`{"id":"m1","name":"Alice","initial":"A","tone":"rose"}`)

	var before struct {
		Seq int `json:"seq"`
	}
	decodeJSON(t, doReq(t, h, http.MethodGet, "/api/snapshot", ""), &before)

	body := `{"tasks":[
		{"title":"Vaisselle","cat":"cuisine","assignee":"alice","due":"2026-07-08T18:00","recurring":false},
		{"title":"Poubelles","cat":"maison","assignee":"Bob","due":"2026-07-08T08:00"},
		{"title":"","cat":"maison"}
	]}`
	resp := doReq(t, h, http.MethodPost, "/api/tasks/bulk", body)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("bulk: %d", resp.StatusCode)
	}
	var result struct {
		Seq      int `json:"seq"`
		Created  int `json:"created"`
		Warnings []struct {
			Index int    `json:"index"`
			Error string `json:"error"`
		} `json:"warnings"`
		Errors []struct {
			Index int    `json:"index"`
			Error string `json:"error"`
		} `json:"errors"`
	}
	decodeJSON(t, resp, &result)

	if result.Created != 2 {
		t.Fatalf("expected 2 created, got %d (%+v)", result.Created, result)
	}
	if len(result.Errors) != 1 || result.Errors[0].Index != 2 {
		t.Fatalf("expected 1 error at index 2, got %+v", result.Errors)
	}
	if len(result.Warnings) != 1 || result.Warnings[0].Index != 1 {
		t.Fatalf("expected 1 warning at index 1 (unknown member Bob), got %+v", result.Warnings)
	}
	if result.Seq != before.Seq+1 {
		t.Fatalf("expected a single seq increment, got %d -> %d", before.Seq, result.Seq)
	}

	resp = doReq(t, h, http.MethodGet, "/api/tasks", "")
	var tasks []model.Task
	decodeJSON(t, resp, &tasks)
	if len(tasks) != 2 {
		t.Fatalf("expected 2 tasks in db, got %d", len(tasks))
	}
	var dishTask *model.Task
	for i := range tasks {
		if tasks[i].Title == "Vaisselle" {
			dishTask = &tasks[i]
		}
	}
	if dishTask == nil || dishTask.Assignee == nil || *dishTask.Assignee != "m1" {
		t.Fatalf("expected Vaisselle assigned to m1, got %+v", dishTask)
	}
}

func TestBulkCreateTasksWithChecklist(t *testing.T) {
	h := newTestHandler(t)
	body := `{"tasks":[
		{"title":"Grand ménage","cat":"maison","checklist":[{"text":"Aspirateur"},{"text":"Serpillère"}]}
	]}`
	resp := doReq(t, h, http.MethodPost, "/api/tasks/bulk", body)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("bulk: %d", resp.StatusCode)
	}

	resp = doReq(t, h, http.MethodGet, "/api/tasks", "")
	var tasks []model.Task
	decodeJSON(t, resp, &tasks)
	if len(tasks) != 1 || len(tasks[0].Checklist) != 2 {
		t.Fatalf("expected 1 task with 2 checklist items, got %+v", tasks)
	}
	for _, item := range tasks[0].Checklist {
		if item.ID == "" {
			t.Fatalf("expected generated checklist item ID, got empty")
		}
	}
}
