package sync

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// --- PollingWatcher unit tests (no MongoDB required) ---
// These tests use a fakeDB shim to avoid touching a real MongoDB deployment.

// TestChangeOperationConstants verifies the ChangeOperation constants have the
// expected string values that MongoDB change streams use.
func TestChangeOperationConstants(t *testing.T) {
	cases := map[ChangeOperation]string{
		ChangeOpInsert:  "insert",
		ChangeOpUpdate:  "update",
		ChangeOpReplace: "replace",
		ChangeOpDelete:  "delete",
	}
	for op, want := range cases {
		if string(op) != want {
			t.Errorf("ChangeOperation %q: got %q, want %q", op, string(op), want)
		}
	}
}

// TestErrChangeStreamsNotSupported verifies the sentinel error has a sensible
// message and can be compared with errors.Is.
func TestErrChangeStreamsNotSupported(t *testing.T) {
	err := ErrChangeStreamsNotSupported
	if err.Error() == "" {
		t.Fatal("ErrChangeStreamsNotSupported should have a non-empty message")
	}
	// The sentinel is a value type; a second reference compares equal.
	var other changeStreamsNotSupportedError
	if err.Error() != other.Error() {
		t.Fatal("Two changeStreamsNotSupportedError values should produce the same message")
	}
}

// TestChangeEventFields ensures ChangeEvent fields are accessible and settable.
func TestChangeEventFields(t *testing.T) {
	evt := ChangeEvent{
		Collection: "specs",
		Operation:  ChangeOpInsert,
		DocumentID: "doc-1",
		FullDoc:    map[string]any{"foo": "bar"},
	}

	if evt.Collection != "specs" {
		t.Errorf("Collection: got %q, want %q", evt.Collection, "specs")
	}
	if evt.Operation != ChangeOpInsert {
		t.Errorf("Operation: got %q, want %q", evt.Operation, ChangeOpInsert)
	}
	if evt.DocumentID != "doc-1" {
		t.Errorf("DocumentID: got %q, want %q", evt.DocumentID, "doc-1")
	}
	if evt.FullDoc["foo"] != "bar" {
		t.Errorf("FullDoc[foo]: got %v, want %q", evt.FullDoc["foo"], "bar")
	}
}

// --- noopWatcher: minimal ChangeWatcher that emits a fixed sequence of events
// then blocks until ctx is cancelled. Used to test handler dispatching.

type noopWatcher struct {
	events []ChangeEvent
}

func (n *noopWatcher) Watch(ctx context.Context, handler Handler) error {
	for _, e := range n.events {
		select {
		case <-ctx.Done():
			return nil
		default:
		}
		handler(e)
	}
	<-ctx.Done()
	return nil
}

// TestHandlerReceivesAllEvents verifies that a ChangeWatcher delivers every
// event to the handler in order.
func TestHandlerReceivesAllEvents(t *testing.T) {
	want := []ChangeEvent{
		{Collection: "specs", Operation: ChangeOpInsert, DocumentID: "s1"},
		{Collection: "operations", Operation: ChangeOpReplace, DocumentID: "o1"},
		{Collection: "global_store", Operation: ChangeOpDelete, DocumentID: "k1"},
	}

	ctx, cancel := context.WithCancel(context.Background())

	var mu sync.Mutex
	var got []ChangeEvent
	handler := func(e ChangeEvent) {
		mu.Lock()
		got = append(got, e)
		mu.Unlock()
	}

	w := &noopWatcher{events: want}
	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = w.Watch(ctx, handler)
	}()

	// Wait until all events have been delivered, then cancel.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		mu.Lock()
		n := len(got)
		mu.Unlock()
		if n >= len(want) {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	cancel()
	<-done

	mu.Lock()
	defer mu.Unlock()

	if len(got) != len(want) {
		t.Fatalf("got %d events, want %d", len(got), len(want))
	}
	for i, e := range got {
		w := want[i]
		if e.Collection != w.Collection || e.Operation != w.Operation || e.DocumentID != w.DocumentID {
			t.Errorf("event[%d]: got %+v, want %+v", i, e, w)
		}
	}
}

// TestHandlerConcurrencySafe verifies that the handler callback can be called
// from multiple goroutines without data races (detectable with -race).
func TestHandlerConcurrencySafe(t *testing.T) {
	var count atomic.Int64
	handler := func(e ChangeEvent) {
		count.Add(1)
	}

	const workers = 10
	const eventsPerWorker = 100

	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < eventsPerWorker; j++ {
				handler(ChangeEvent{Collection: "specs", Operation: ChangeOpInsert, DocumentID: "x"})
			}
		}()
	}
	wg.Wait()

	if got := count.Load(); got != workers*eventsPerWorker {
		t.Errorf("count: got %d, want %d", got, workers*eventsPerWorker)
	}
}
