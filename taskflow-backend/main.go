package main

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	_ "embed"
	"log"
	"net/http"
	"os"

	chi "github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"

	"foyer/taskflow/internal/api"
	"foyer/taskflow/internal/db"
	"foyer/taskflow/internal/model"
	"foyer/taskflow/internal/mqtt"
	"foyer/taskflow/internal/ws"
)

//go:embed web/foyer-tasks-card.js
var cardJS []byte

const (
	cardDest    = "/config/www/taskflow/foyer-tasks-card.js"
	cardURL     = "/local/taskflow/foyer-tasks-card.js"
	supervisorAPI = "http://supervisor/core/api"
)

func main() {
	dbPath    := env("FOYER_DB", "./foyer.sqlite")
	port      := env("PORT", "8787")
	mqttURL   := env("FOYER_MQTT_URL", "mqtt://localhost:1883")
	staticDir := env("FOYER_STATIC", "./public")
	secret    := env("FOYER_SECRET", "")

	database, err := db.Open(dbPath)
	if err != nil {
		log.Fatalf("db: %v", err)
	}
	seedFromOptions(database)
	bootstrap(env("SUPERVISOR_TOKEN", ""))

	hub := ws.NewHub()

	mqttClient := mqtt.NewClient(mqttURL)
	if err := mqttClient.Connect(); err != nil {
		log.Printf("MQTT unavailable (continuing without): %v", err)
		mqttClient = nil
	}

	h := api.NewHandler(database, hub, mqttClient, staticDir, secret)

	r := chi.NewRouter()
	r.Use(chimw.Logger)
	r.Use(chimw.Recoverer)
	r.Use(corsMiddleware)
	h.Mount(r)

	log.Printf("foyer-go listening on :%s", port)
	log.Fatal(http.ListenAndServe(":"+port, r))
}

// bootstrap copies the Lovelace card to /config/www/taskflow/ and registers
// it as a Lovelace resource via the HA Supervisor API.
func bootstrap(token string) {
	if err := os.MkdirAll("/config/www/taskflow", 0o755); err != nil {
		log.Printf("bootstrap: mkdir: %v", err)
		return
	}
	if err := os.WriteFile(cardDest, cardJS, 0o644); err != nil {
		log.Printf("bootstrap: write card: %v", err)
		return
	}
	log.Printf("bootstrap: card écrite dans %s", cardDest)

	if token == "" {
		log.Printf("bootstrap: SUPERVISOR_TOKEN absent, enregistrement Lovelace ignoré")
		return
	}
	registerLovelaceResource(token, cardURL)
}

func registerLovelaceResource(token, url string) {
	client := &http.Client{}

	// Vérifier si la ressource est déjà enregistrée
	req, _ := http.NewRequest("GET", supervisorAPI+"/lovelace/resources", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("bootstrap: HA API injoignable: %v", err)
		return
	}
	defer resp.Body.Close()

	var resources []struct {
		URL string `json:"url"`
	}
	json.NewDecoder(resp.Body).Decode(&resources)
	for _, r := range resources {
		if r.URL == url {
			log.Printf("bootstrap: ressource Lovelace déjà enregistrée")
			return
		}
	}

	// Enregistrer
	payload, _ := json.Marshal(map[string]string{"url": url, "res_type": "module"})
	req, _ = http.NewRequest("POST", supervisorAPI+"/lovelace/resources", bytes.NewReader(payload))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	resp2, err := client.Do(req)
	if err != nil {
		log.Printf("bootstrap: enregistrement ressource: %v", err)
		return
	}
	defer resp2.Body.Close()
	log.Printf("bootstrap: ressource Lovelace enregistrée (%s)", url)
}

type haOptions struct {
	HouseholdName string `json:"household_name"`
	Members       []struct {
		Name   string `json:"name"`
		Tone   string `json:"tone"`
		Avatar string `json:"avatar"`
	} `json:"members"`
	Pets []struct {
		Name    string `json:"name"`
		Species string `json:"species"`
		Emoji   string `json:"emoji"`
	} `json:"pets"`
}

func seedFromOptions(database *db.DB) {
	f, err := os.Open(env("FOYER_OPTIONS", "/data/options.json"))
	if err != nil {
		return
	}
	defer f.Close()

	var opts haOptions
	if err := json.NewDecoder(f).Decode(&opts); err != nil {
		log.Printf("seed: cannot parse options.json: %v", err)
		return
	}

	empty, err := database.IsEmpty()
	if err != nil || !empty {
		return
	}

	for _, m := range opts.Members {
		initial := ""
		if len([]rune(m.Name)) > 0 {
			initial = string([]rune(m.Name)[0])
		}
		if err := database.UpsertMember(model.Member{
			ID:      newID(),
			Name:    m.Name,
			Initial: initial,
			Tone:    model.Tone(m.Tone),
			Avatar:  m.Avatar,
		}); err != nil {
			log.Printf("seed: member %s: %v", m.Name, err)
		}
	}

	for _, p := range opts.Pets {
		if err := database.UpsertPet(model.Pet{
			ID:      newID(),
			Name:    p.Name,
			Species: p.Species,
			Emoji:   p.Emoji,
		}); err != nil {
			log.Printf("seed: pet %s: %v", p.Name, err)
		}
	}

	if opts.HouseholdName != "" {
		if s, err := database.GetSettings(); err == nil {
			s.HouseholdName = opts.HouseholdName
			database.SaveSettings(s)
		}
	}

	log.Printf("seed: %d membre(s), %d animal(aux) depuis options.json", len(opts.Members), len(opts.Pets))
}

func newID() string {
	b := make([]byte, 9)
	_, _ = rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PATCH, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, x-foyer-secret")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
