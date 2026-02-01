package tracing

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
)

// WebSocketHandler handles WebSocket connections for live tracing
type WebSocketHandler struct {
	service  *Service
	upgrader websocket.Upgrader
}

// NewWebSocketHandler creates a new WebSocket handler
func NewWebSocketHandler(service *Service) *WebSocketHandler {
	return &WebSocketHandler{
		service: service,
		upgrader: websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			CheckOrigin: func(r *http.Request) bool {
				return true // Allow all origins for development
			},
		},
	}
}

// ServeHTTP handles WebSocket upgrade and streaming
func (h *WebSocketHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Upgrade HTTP connection to WebSocket
	conn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade failed: %v", err)
		return
	}
	defer conn.Close()

	// Subscribe to trace events
	subID, traceChan := h.service.Subscribe()
	defer h.service.Unsubscribe(subID)

	// Set up ping/pong for keepalive
	conn.SetPongHandler(func(string) error {
		conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	// Start a goroutine to read messages (for handling close)
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			_, _, err := conn.ReadMessage()
			if err != nil {
				return
			}
		}
	}()

	// Start ticker for ping
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	// Stream traces to client
	for {
		select {
		case trace, ok := <-traceChan:
			if !ok {
				return
			}

			// Serialize trace to JSON
			data, err := json.Marshal(trace)
			if err != nil {
				log.Printf("Failed to marshal trace: %v", err)
				continue
			}

			// Send to client
			if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
				log.Printf("Failed to send trace: %v", err)
				return
			}

		case <-ticker.C:
			// Send ping
			if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}

		case <-done:
			return
		}
	}
}
