package ws_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"foyer/taskflow/internal/ws"
	"github.com/gorilla/websocket"
)

func TestBroadcast(t *testing.T) {
	hub := ws.NewHub()
	srv := httptest.NewServer(http.HandlerFunc(hub.ServeHTTP))
	defer srv.Close()

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http")
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	// Laisser le temps au hub d'enregistrer le client
	time.Sleep(50 * time.Millisecond)

	hub.Broadcast(42)

	conn.SetReadDeadline(time.Now().Add(time.Second))
	_, msg, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("ReadMessage: %v", err)
	}
	var payload map[string]int
	if err := json.Unmarshal(msg, &payload); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if payload["seq"] != 42 {
		t.Fatalf("expected seq=42, got %d", payload["seq"])
	}
}
