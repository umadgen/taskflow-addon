package ws

import (
	"encoding/json"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
)

type Hub struct {
	clients  map[*websocket.Conn]struct{}
	mu       sync.Mutex
	upgrader websocket.Upgrader
	onChange chan struct{}
}

func NewHub() *Hub {
	return &Hub{
		clients:  make(map[*websocket.Conn]struct{}),
		onChange: make(chan struct{}, 1),
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool { return true },
		},
	}
}

func (h *Hub) OnChange() <-chan struct{} {
	return h.onChange
}

func (h *Hub) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	conn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	h.mu.Lock()
	h.clients[conn] = struct{}{}
	h.mu.Unlock()

	for {
		if _, _, err := conn.ReadMessage(); err != nil {
			break
		}
	}

	h.mu.Lock()
	delete(h.clients, conn)
	h.mu.Unlock()
	conn.Close()
}

func (h *Hub) Broadcast(seq int) {
	msg, _ := json.Marshal(map[string]int{"seq": seq})
	h.mu.Lock()
	for conn := range h.clients {
		_ = conn.WriteMessage(websocket.TextMessage, msg)
	}
	h.mu.Unlock()

	select {
	case h.onChange <- struct{}{}:
	default:
	}
}
