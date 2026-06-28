package db

import (
	"database/sql"
	"encoding/json"
	"fmt"

	"foyer/taskflow/internal/model"
	_ "modernc.org/sqlite"
)

type DB struct {
	sql *sql.DB
}

func Open(path string) (*DB, error) {
	dsn := "file:" + path + "?_pragma=journal_mode(WAL)&_pragma=foreign_keys(ON)"
	if path == ":memory:" {
		dsn = "file::memory:?_pragma=journal_mode(WAL)"
	}
	sqldb, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	sqldb.SetMaxOpenConns(1)
	d := &DB{sql: sqldb}
	return d, d.migrate()
}

func (d *DB) migrate() error {
	_, err := d.sql.Exec(`
		CREATE TABLE IF NOT EXISTS members (
			id      TEXT PRIMARY KEY,
			name    TEXT NOT NULL,
			initial TEXT NOT NULL,
			tone    TEXT NOT NULL,
			avatar  TEXT NOT NULL DEFAULT ''
		);
		CREATE TABLE IF NOT EXISTS tasks (
			id        TEXT PRIMARY KEY,
			title     TEXT NOT NULL,
			cat       TEXT NOT NULL,
			assignee  TEXT,
			done      INTEGER NOT NULL DEFAULT 0,
			done_by   TEXT,
			done_at   TEXT,
			due       TEXT NOT NULL,
			late      INTEGER NOT NULL DEFAULT 0,
			recurring INTEGER NOT NULL DEFAULT 0,
			repeat    TEXT,
			week_days TEXT,
			month_day INTEGER,
			time      TEXT,
			freq_text TEXT
		);
		CREATE TABLE IF NOT EXISTS history (
			id      TEXT PRIMARY KEY,
			title   TEXT NOT NULL,
			cat     TEXT NOT NULL,
			by      TEXT NOT NULL,
			at      TEXT NOT NULL,
			task_id TEXT
		);
		CREATE TABLE IF NOT EXISTS pets (
			id   TEXT PRIMARY KEY,
			data TEXT NOT NULL
		);
		CREATE TABLE IF NOT EXISTS settings (
			id   INTEGER PRIMARY KEY CHECK (id = 1),
			data TEXT NOT NULL
		);
		CREATE TABLE IF NOT EXISTS seq (
			id  INTEGER PRIMARY KEY CHECK (id = 1),
			val INTEGER NOT NULL DEFAULT 0
		);
		INSERT OR IGNORE INTO seq (id, val) VALUES (1, 0);
		INSERT OR IGNORE INTO settings (id, data) VALUES (1, '{"theme":"dark","accent":"#6366f1","notifTasks":false,"notifPets":false,"notifAgenda":false,"sounds":false,"currentMember":"","householdName":"Foyer","onboarded":false}');
	`)
	if err != nil {
		return err
	}
	// Migrations for existing DBs (errors ignored when column already present)
	d.sql.Exec(`ALTER TABLE members ADD COLUMN avatar TEXT NOT NULL DEFAULT ''`)
	d.sql.Exec(`ALTER TABLE tasks ADD COLUMN checklist TEXT NOT NULL DEFAULT '[]'`)
	return nil
}

// ── Seq ──────────────────────────────────────────────────────────────────────

func (d *DB) GetSeq() (int, error) {
	var v int
	return v, d.sql.QueryRow(`SELECT val FROM seq WHERE id = 1`).Scan(&v)
}

func (d *DB) IncrSeq() (int, error) {
	var v int
	return v, d.sql.QueryRow(`UPDATE seq SET val = val + 1 WHERE id = 1 RETURNING val`).Scan(&v)
}

// ── Members ───────────────────────────────────────────────────────────────────

func (d *DB) GetMembers() ([]model.Member, error) {
	rows, err := d.sql.Query(`SELECT id, name, initial, tone, avatar FROM members ORDER BY rowid`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.Member
	for rows.Next() {
		var m model.Member
		if err := rows.Scan(&m.ID, &m.Name, &m.Initial, &m.Tone, &m.Avatar); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	if out == nil {
		out = []model.Member{}
	}
	return out, nil
}

func (d *DB) UpsertMember(m model.Member) error {
	_, err := d.sql.Exec(
		`INSERT INTO members (id, name, initial, tone, avatar) VALUES (?, ?, ?, ?, ?)
		 ON CONFLICT(id) DO UPDATE SET name=excluded.name, initial=excluded.initial, tone=excluded.tone, avatar=excluded.avatar`,
		m.ID, m.Name, m.Initial, m.Tone, m.Avatar,
	)
	return err
}

func (d *DB) DeleteMember(id string) error {
	_, err := d.sql.Exec(`DELETE FROM members WHERE id = ?`, id)
	return err
}

// ── Tasks ─────────────────────────────────────────────────────────────────────

func (d *DB) GetTasks() ([]model.Task, error) {
	rows, err := d.sql.Query(`
		SELECT id, title, cat, assignee, done, done_by, done_at,
		       due, late, recurring, repeat, week_days, month_day, time, freq_text, checklist
		FROM tasks ORDER BY due`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.Task
	for rows.Next() {
		t, err := scanTask(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	if out == nil {
		out = []model.Task{}
	}
	return out, nil
}

func (d *DB) GetTask(id string) (*model.Task, error) {
	rows, err := d.sql.Query(`
		SELECT id, title, cat, assignee, done, done_by, done_at,
		       due, late, recurring, repeat, week_days, month_day, time, freq_text, checklist
		FROM tasks WHERE id = ?`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	if !rows.Next() {
		return nil, nil
	}
	t, err := scanTask(rows)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func scanTask(rows *sql.Rows) (model.Task, error) {
	var t model.Task
	var done, late, recurring int
	var weekDaysJSON, checklistJSON sql.NullString
	err := rows.Scan(
		&t.ID, &t.Title, &t.Cat,
		&t.Assignee, &done, &t.DoneBy, &t.DoneAt,
		&t.Due, &late, &recurring,
		&t.Repeat, &weekDaysJSON, &t.MonthDay, &t.Time, &t.FreqText,
		&checklistJSON,
	)
	if err != nil {
		return t, err
	}
	t.Done = done != 0
	t.Late = late != 0
	t.Recurring = recurring != 0
	if weekDaysJSON.Valid && weekDaysJSON.String != "" && weekDaysJSON.String != "null" {
		_ = json.Unmarshal([]byte(weekDaysJSON.String), &t.WeekDays)
	}
	if t.WeekDays == nil {
		t.WeekDays = []int{}
	}
	if checklistJSON.Valid && checklistJSON.String != "" && checklistJSON.String != "null" {
		_ = json.Unmarshal([]byte(checklistJSON.String), &t.Checklist)
	}
	if t.Checklist == nil {
		t.Checklist = []model.ChecklistItem{}
	}
	return t, nil
}

func (d *DB) UpsertTask(t model.Task) error {
	wdJSON, _ := json.Marshal(t.WeekDays)
	clJSON, _ := json.Marshal(t.Checklist)
	_, err := d.sql.Exec(`
		INSERT INTO tasks
		  (id, title, cat, assignee, done, done_by, done_at, due, late, recurring, repeat, week_days, month_day, time, freq_text, checklist)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
		  title=excluded.title, cat=excluded.cat, assignee=excluded.assignee,
		  done=excluded.done, done_by=excluded.done_by, done_at=excluded.done_at,
		  due=excluded.due, late=excluded.late, recurring=excluded.recurring,
		  repeat=excluded.repeat, week_days=excluded.week_days, month_day=excluded.month_day,
		  time=excluded.time, freq_text=excluded.freq_text, checklist=excluded.checklist`,
		t.ID, t.Title, t.Cat,
		t.Assignee, boolInt(t.Done), t.DoneBy, t.DoneAt,
		t.Due, boolInt(t.Late), boolInt(t.Recurring),
		t.Repeat, string(wdJSON), t.MonthDay, t.Time, t.FreqText, string(clJSON),
	)
	return err
}

func (d *DB) DeleteTask(id string) error {
	_, err := d.sql.Exec(`DELETE FROM tasks WHERE id = ?`, id)
	return err
}

// ── History ───────────────────────────────────────────────────────────────────

func (d *DB) GetHistory() ([]model.HistoryEntry, error) {
	rows, err := d.sql.Query(
		`SELECT id, title, cat, by, at, task_id FROM history ORDER BY at DESC LIMIT 200`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.HistoryEntry
	for rows.Next() {
		var e model.HistoryEntry
		var taskID sql.NullString
		if err := rows.Scan(&e.ID, &e.Title, &e.Cat, &e.By, &e.At, &taskID); err != nil {
			return nil, err
		}
		if taskID.Valid {
			e.TaskID = taskID.String
		}
		out = append(out, e)
	}
	if out == nil {
		out = []model.HistoryEntry{}
	}
	return out, nil
}

func (d *DB) InsertHistory(e model.HistoryEntry) error {
	_, err := d.sql.Exec(
		`INSERT OR IGNORE INTO history (id, title, cat, by, at, task_id) VALUES (?, ?, ?, ?, ?, ?)`,
		e.ID, e.Title, e.Cat, e.By, e.At, nullStr(e.TaskID),
	)
	return err
}

// ── Pets ──────────────────────────────────────────────────────────────────────

func (d *DB) GetPets() ([]model.Pet, error) {
	rows, err := d.sql.Query(`SELECT data FROM pets ORDER BY rowid`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.Pet
	for rows.Next() {
		var data string
		if err := rows.Scan(&data); err != nil {
			return nil, err
		}
		var p model.Pet
		if err := json.Unmarshal([]byte(data), &p); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	if out == nil {
		out = []model.Pet{}
	}
	return out, nil
}

func (d *DB) GetPet(id string) (*model.Pet, error) {
	var data string
	err := d.sql.QueryRow(`SELECT data FROM pets WHERE id = ?`, id).Scan(&data)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var p model.Pet
	return &p, json.Unmarshal([]byte(data), &p)
}

func (d *DB) UpsertPet(p model.Pet) error {
	data, err := json.Marshal(p)
	if err != nil {
		return err
	}
	_, err = d.sql.Exec(
		`INSERT INTO pets (id, data) VALUES (?, ?) ON CONFLICT(id) DO UPDATE SET data=excluded.data`,
		p.ID, string(data),
	)
	return err
}

func (d *DB) DeletePet(id string) error {
	_, err := d.sql.Exec(`DELETE FROM pets WHERE id = ?`, id)
	return err
}

// ── Settings ──────────────────────────────────────────────────────────────────

func (d *DB) GetSettings() (model.Settings, error) {
	var data string
	if err := d.sql.QueryRow(`SELECT data FROM settings WHERE id = 1`).Scan(&data); err != nil {
		return model.Settings{}, err
	}
	var s model.Settings
	return s, json.Unmarshal([]byte(data), &s)
}

func (d *DB) SaveSettings(s model.Settings) error {
	data, err := json.Marshal(s)
	if err != nil {
		return err
	}
	_, err = d.sql.Exec(
		`INSERT INTO settings (id, data) VALUES (1, ?) ON CONFLICT(id) DO UPDATE SET data=excluded.data`,
		string(data),
	)
	return err
}

// ── Snapshot MQTT ─────────────────────────────────────────────────────────────

func (d *DB) GetSnapshot() (model.MQTTSnapshot, error) {
	tasks, err := d.GetTasks()
	if err != nil {
		return model.MQTTSnapshot{}, fmt.Errorf("tasks: %w", err)
	}
	members, err := d.GetMembers()
	if err != nil {
		return model.MQTTSnapshot{}, fmt.Errorf("members: %w", err)
	}
	history, err := d.GetHistory()
	if err != nil {
		return model.MQTTSnapshot{}, fmt.Errorf("history: %w", err)
	}
	seq, err := d.GetSeq()
	if err != nil {
		return model.MQTTSnapshot{}, fmt.Errorf("seq: %w", err)
	}
	return model.MQTTSnapshot{Seq: seq, Tasks: tasks, Members: members, History: history}, nil
}

// ── Import ────────────────────────────────────────────────────────────────────

func (d *DB) ClearAll() error {
	tx, err := d.sql.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, table := range []string{"members", "tasks", "history", "pets", "settings"} {
		if _, err := tx.Exec(`DELETE FROM ` + table); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (d *DB) IsEmpty() (bool, error) {
	var n int
	err := d.sql.QueryRow(`SELECT COUNT(*) FROM members`).Scan(&n)
	if err != nil {
		return false, err
	}
	return n == 0, nil
}

func (d *DB) ImportData(data model.AppData) error {
	tx, err := d.sql.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	for _, m := range data.Members {
		if _, err := tx.Exec(
			`INSERT OR REPLACE INTO members (id, name, initial, tone) VALUES (?, ?, ?, ?)`,
			m.ID, m.Name, m.Initial, m.Tone,
		); err != nil {
			return fmt.Errorf("member %s: %w", m.ID, err)
		}
	}
	for _, t := range data.Tasks {
		wdJSON, _ := json.Marshal(t.WeekDays)
		if _, err := tx.Exec(`
			INSERT OR REPLACE INTO tasks
			  (id, title, cat, assignee, done, done_by, done_at, due, late, recurring, repeat, week_days, month_day, time, freq_text)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			t.ID, t.Title, t.Cat,
			t.Assignee, boolInt(t.Done), t.DoneBy, t.DoneAt,
			t.Due, boolInt(t.Late), boolInt(t.Recurring),
			t.Repeat, string(wdJSON), t.MonthDay, t.Time, t.FreqText,
		); err != nil {
			return fmt.Errorf("task %s: %w", t.ID, err)
		}
	}
	for _, e := range data.History {
		if _, err := tx.Exec(
			`INSERT OR REPLACE INTO history (id, title, cat, by, at, task_id) VALUES (?, ?, ?, ?, ?, ?)`,
			e.ID, e.Title, e.Cat, e.By, e.At, nullStr(e.TaskID),
		); err != nil {
			return fmt.Errorf("history %s: %w", e.ID, err)
		}
	}
	for _, p := range data.Pets {
		pdata, _ := json.Marshal(p)
		if _, err := tx.Exec(
			`INSERT OR REPLACE INTO pets (id, data) VALUES (?, ?)`,
			p.ID, string(pdata),
		); err != nil {
			return fmt.Errorf("pet %s: %w", p.ID, err)
		}
	}
	sdata, _ := json.Marshal(data.Settings)
	if _, err := tx.Exec(
		`INSERT INTO settings (id, data) VALUES (1, ?) ON CONFLICT(id) DO UPDATE SET data=excluded.data`,
		string(sdata),
	); err != nil {
		return fmt.Errorf("settings: %w", err)
	}

	return tx.Commit()
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func nullStr(s string) sql.NullString {
	if s == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: s, Valid: true}
}
