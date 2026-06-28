package api

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"path/filepath"

	chi "github.com/go-chi/chi/v5"

	"foyer/taskflow/internal/db"
	"foyer/taskflow/internal/model"
	"foyer/taskflow/internal/mqtt"
	"foyer/taskflow/internal/ws"
)

type Handler struct {
	db     *db.DB
	hub    *ws.Hub
	mqtt   *mqtt.Client
	static string
	secret string
}

func NewHandler(d *db.DB, hub *ws.Hub, mqttClient *mqtt.Client, static, secret string) *Handler {
	return &Handler{db: d, hub: hub, mqtt: mqttClient, static: static, secret: secret}
}

func (h *Handler) Mount(r chi.Router) {
	r.Route("/api", func(r chi.Router) {
		r.Get("/snapshot", h.getSnapshot)
		r.Get("/sync", h.hub.ServeHTTP)
		r.Post("/ops", h.handleOps)

		r.Route("/members", func(r chi.Router) {
			r.Get("/", h.listMembers)
			r.Post("/", h.createMember)
			r.Patch("/{id}", h.updateMember)
			r.Delete("/{id}", h.deleteMember)
		})
		r.Route("/tasks", func(r chi.Router) {
			r.Get("/", h.listTasks)
			r.Post("/", h.createTask)
			r.Patch("/{id}", h.updateTask)
			r.Delete("/{id}", h.deleteTask)
			r.Post("/{id}/complete", h.completeTask)
			r.Post("/{id}/uncomplete", h.uncompleteTask)
		})
		r.Get("/history", h.listHistory)
		r.Route("/pets", func(r chi.Router) {
			r.Get("/", h.listPets)
			r.Post("/", h.createPet)
			r.Patch("/{id}", h.updatePet)
			r.Delete("/{id}", h.deletePet)
			r.Post("/{id}/treatments/{tid}/complete", h.completeTreatment)
			r.Post("/{id}/vaccines/{vid}/complete", h.completeVaccine)
		})
		r.Get("/settings", h.getSettings)
		r.Patch("/settings", h.updateSettings)
		r.Post("/import", h.importData)
		r.Get("/export", h.exportData)
		r.Post("/restore", h.restoreData)
	})

	if h.static != "" {
		fs := http.FileServer(http.Dir(h.static))
		r.Get("/*", func(w http.ResponseWriter, r *http.Request) {
			path := filepath.Join(h.static, filepath.Clean("/"+r.URL.Path))
			if _, err := os.Stat(path); os.IsNotExist(err) {
				http.ServeFile(w, r, filepath.Join(h.static, "index.html"))
				return
			}
			fs.ServeHTTP(w, r)
		})
	}
}

// ── Snapshot ──────────────────────────────────────────────────────────────────

type snapshotResponse struct {
	Data model.AppData `json:"data"`
	Seq  int           `json:"seq"`
}

func (h *Handler) getSnapshot(w http.ResponseWriter, r *http.Request) {
	tasks, _ := h.db.GetTasks()
	members, _ := h.db.GetMembers()
	history, _ := h.db.GetHistory()
	pets, _ := h.db.GetPets()
	settings, _ := h.db.GetSettings()
	seq, _ := h.db.GetSeq()

	writeJSON(w, http.StatusOK, snapshotResponse{
		Data: model.AppData{
			Version:      1,
			Members:      members,
			Pets:         pets,
			Tasks:        tasks,
			History:      history,
			GoogleEvents: []model.GoogleEvent{},
			Settings:     settings,
		},
		Seq: seq,
	})
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func (h *Handler) notify(seq int) {
	snap, err := h.db.GetSnapshot()
	if err != nil {
		log.Printf("GetSnapshot: %v", err)
		return
	}
	if h.mqtt != nil {
		if err := h.mqtt.PublishSnapshot(snap); err != nil {
			log.Printf("MQTT publish: %v", err)
		}
	}
	h.hub.Broadcast(seq)
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, map[string]string{"error": msg})
}

func newID() string {
	b := make([]byte, 9)
	_, _ = rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}
