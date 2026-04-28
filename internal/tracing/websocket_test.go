package tracing

import (
	"bytes"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/prasenjit/go-virtual/internal/models"
)

func TestWebSocketHandler_SendsTraces(t *testing.T) {
	svc := NewService(10)
	handler := NewWebSocketHandler(svc)

	srv := httptest.NewServer(handler)
	defer srv.Close()

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http")
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("failed to dial websocket: %v", err)
	}
	defer conn.Close()

	// Wait for the handler goroutine to subscribe before recording the trace.
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		stats := svc.GetStats()
		if n, _ := stats["activeSubscribers"].(int); n > 0 {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}

	trace := &models.Trace{
		SpecID:      "spec-1",
		OperationID: "op-1",
		Request:     models.TraceRequest{Method: "GET", Path: "/users"},
		Response:    models.TraceResponse{StatusCode: 200},
	}
	svc.RecordTrace(trace)

	_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	_, message, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("failed to read message: %v", err)
	}
	if !bytes.Contains(message, []byte("\"specId\"")) {
		t.Fatalf("expected trace payload, got %s", string(message))
	}
}
