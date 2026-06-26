package db_test

import (
	"testing"

	"foyer/taskflow/internal/db"
	"foyer/taskflow/internal/model"
)

func openMemory(t *testing.T) *db.DB {
	t.Helper()
	d, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	return d
}

func TestOpen(t *testing.T) {
	d := openMemory(t)
	if d == nil {
		t.Fatal("expected non-nil DB")
	}
}

func TestSeq(t *testing.T) {
	d := openMemory(t)
	seq, err := d.GetSeq()
	if err != nil {
		t.Fatalf("GetSeq: %v", err)
	}
	if seq != 0 {
		t.Fatalf("expected seq=0, got %d", seq)
	}
	seq, err = d.IncrSeq()
	if err != nil {
		t.Fatalf("IncrSeq: %v", err)
	}
	if seq != 1 {
		t.Fatalf("expected seq=1 after incr, got %d", seq)
	}
}

func TestMembersRoundtrip(t *testing.T) {
	d := openMemory(t)
	m := model.Member{ID: "m1", Name: "Alice", Initial: "A", Tone: model.ToneRose}
	if err := d.UpsertMember(m); err != nil {
		t.Fatalf("UpsertMember: %v", err)
	}
	members, err := d.GetMembers()
	if err != nil {
		t.Fatalf("GetMembers: %v", err)
	}
	if len(members) != 1 || members[0].ID != "m1" {
		t.Fatalf("unexpected members: %+v", members)
	}
	if err := d.DeleteMember("m1"); err != nil {
		t.Fatalf("DeleteMember: %v", err)
	}
	members, _ = d.GetMembers()
	if len(members) != 0 {
		t.Fatalf("expected empty after delete, got %d", len(members))
	}
}

func TestTasksRoundtrip(t *testing.T) {
	d := openMemory(t)
	assignee := "m1"
	weekDays := []int{0, 2}
	task := model.Task{
		ID:        "t1",
		Title:     "Faire la vaisselle",
		Cat:       "maison",
		Assignee:  &assignee,
		Due:       "2026-06-27T18:00",
		Recurring: true,
		Repeat:    strPtr("semaine"),
		WeekDays:  weekDays,
	}
	if err := d.UpsertTask(task); err != nil {
		t.Fatalf("UpsertTask: %v", err)
	}
	tasks, err := d.GetTasks()
	if err != nil {
		t.Fatalf("GetTasks: %v", err)
	}
	if len(tasks) != 1 || tasks[0].ID != "t1" {
		t.Fatalf("unexpected tasks: %+v", tasks)
	}
	if len(tasks[0].WeekDays) != 2 || tasks[0].WeekDays[0] != 0 {
		t.Fatalf("weekDays not preserved: %+v", tasks[0].WeekDays)
	}
	got, err := d.GetTask("t1")
	if err != nil {
		t.Fatalf("GetTask: %v", err)
	}
	if got == nil || got.Title != "Faire la vaisselle" {
		t.Fatalf("GetTask result: %+v", got)
	}
	if err := d.DeleteTask("t1"); err != nil {
		t.Fatalf("DeleteTask: %v", err)
	}
	tasks, _ = d.GetTasks()
	if len(tasks) != 0 {
		t.Fatalf("expected empty after delete")
	}
}

func TestHistoryInsert(t *testing.T) {
	d := openMemory(t)
	e := model.HistoryEntry{ID: "h1", Title: "Test", Cat: "maison", By: "m1", At: "2026-06-26T10:00:00Z"}
	if err := d.InsertHistory(e); err != nil {
		t.Fatalf("InsertHistory: %v", err)
	}
	entries, err := d.GetHistory()
	if err != nil {
		t.Fatalf("GetHistory: %v", err)
	}
	if len(entries) != 1 || entries[0].ID != "h1" {
		t.Fatalf("unexpected entries: %+v", entries)
	}
}

func TestIsEmpty(t *testing.T) {
	d := openMemory(t)
	empty, err := d.IsEmpty()
	if err != nil {
		t.Fatalf("IsEmpty: %v", err)
	}
	if !empty {
		t.Fatal("expected empty on fresh DB")
	}
	d.UpsertMember(model.Member{ID: "m1", Name: "Alice", Initial: "A", Tone: model.ToneRose})
	empty, _ = d.IsEmpty()
	if empty {
		t.Fatal("expected non-empty after insert")
	}
}

func strPtr(s string) *string { return &s }
