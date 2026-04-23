package proxy

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/prasenjit/go-virtual/internal/models"
	"github.com/prasenjit/go-virtual/internal/storage"
)

// startFakeBackend starts a test HTTP server that returns the configured
// status, headers, and body for any request.
func startFakeBackend(statusCode int, body string, headers map[string]string) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		for k, v := range headers {
			w.Header().Set(k, v)
		}
		w.WriteHeader(statusCode)
		fmt.Fprint(w, body)
	}))
}

func makeTestRecorder(t *testing.T) (*Recorder, storage.Storage) {
	t.Helper()
	store := storage.NewMemoryStorage()
	return NewRecorder(store), store
}

type barrierStorage struct {
	storage.Storage
	mu      sync.Mutex
	waiting int
	release chan struct{}
}

func newBarrierStorage(inner storage.Storage) *barrierStorage {
	return &barrierStorage{
		Storage: inner,
		release: make(chan struct{}),
	}
}

func (b *barrierStorage) GetResponseConfigsByOperation(opID string) ([]*models.ResponseConfig, error) {
	b.mu.Lock()
	b.waiting++
	if b.waiting == 2 {
		close(b.release)
	}
	release := b.release
	b.mu.Unlock()

	select {
	case <-release:
	case <-time.After(250 * time.Millisecond):
	}

	return b.Storage.GetResponseConfigsByOperation(opID)
}

// ---- ProxyAndRecord ----

func TestProxyAndRecord_BasicForward(t *testing.T) {
	backend := startFakeBackend(200, `{"hello":"world"}`, map[string]string{
		"Content-Type": "application/json",
		"X-Backend":    "yes",
	})
	defer backend.Close()

	rec, _ := makeTestRecorder(t)

	spec := &models.Spec{BackendURI: backend.URL}
	op := &models.Operation{ID: "op-1"}

	status, headers, body, err := rec.ProxyAndRecord("GET", "/items", "", http.Header{}, "", op, spec, "sig1")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status != 200 {
		t.Errorf("expected status 200, got %d", status)
	}
	if body != `{"hello":"world"}` {
		t.Errorf("unexpected body: %q", body)
	}
	if headers["Content-Type"] != "application/json" {
		t.Errorf("expected Content-Type header, got %v", headers)
	}
	if _, ok := headers["X-Backend"]; !ok {
		t.Error("expected X-Backend header to be forwarded")
	}
}

func TestProxyAndRecord_StripHopByHopResponseHeaders(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Set headers that are in skipRecordedResponseHeaders and should NOT appear in stored headers
		w.Header().Set("Date", "Mon, 01 Jan 2024 00:00:00 GMT")
		w.Header().Set("Age", "123")
		w.Header().Set("Via", "1.1 proxy")
		w.Header().Set("Alt-Svc", `h3=":443"`)
		w.Header().Set("X-Keep", "this")
		w.WriteHeader(200)
		fmt.Fprint(w, "hello")
	}))
	defer backend.Close()

	rec, _ := makeTestRecorder(t)
	spec := &models.Spec{BackendURI: backend.URL}
	op := &models.Operation{ID: "op-1"}

	_, headers, _, err := rec.ProxyAndRecord("GET", "/", "", http.Header{}, "", op, spec, "sig1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	skipped := []string{"Date", "Age", "Via", "Alt-Svc"}
	for _, h := range skipped {
		if _, ok := headers[h]; ok {
			t.Errorf("header %q should have been stripped from recorded headers", h)
		}
	}
	if headers["X-Keep"] != "this" {
		t.Errorf("expected X-Keep to be kept, got %v", headers)
	}
}

func TestProxyAndRecord_BackendNotReachable(t *testing.T) {
	rec, _ := makeTestRecorder(t)
	spec := &models.Spec{BackendURI: "http://127.0.0.1:1"} // nothing listening there
	op := &models.Operation{ID: "op-1"}

	_, _, _, err := rec.ProxyAndRecord("GET", "/", "", http.Header{}, "", op, spec, "sig1")
	if err == nil {
		t.Fatal("expected an error for unreachable backend")
	}
}

func TestProxyAndRecord_WithQueryAndBasePath(t *testing.T) {
	var receivedPath string
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedPath = r.URL.RequestURI()
		w.WriteHeader(200)
	}))
	defer backend.Close()

	rec, _ := makeTestRecorder(t)
	spec := &models.Spec{BackendURI: backend.URL, BasePath: "/v1"}
	op := &models.Operation{ID: "op-1"}

	rec.ProxyAndRecord("GET", "/v1/pets", "q=dog", http.Header{}, "", op, spec, "sig1")

	if receivedPath != "/pets?q=dog" {
		t.Errorf("expected backend to receive /pets?q=dog, got %q", receivedPath)
	}
}

func TestProxyAndRecord_StripRequestHopByHopHeaders(t *testing.T) {
	var receivedConnection string
	var receivedKeepAlive string
	var receivedCustom string
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedConnection = r.Header.Get("Connection")
		receivedKeepAlive = r.Header.Get("Keep-Alive")
		receivedCustom = r.Header.Get("X-Custom")
		w.WriteHeader(200)
	}))
	defer backend.Close()

	rec, _ := makeTestRecorder(t)
	spec := &models.Spec{BackendURI: backend.URL}
	op := &models.Operation{ID: "op-1"}

	reqHeaders := http.Header{
		"Connection": {"keep-alive"},
		"Keep-Alive": {"timeout=5"},
		"X-Custom":   {"kept"},
	}
	rec.ProxyAndRecord("GET", "/", "", reqHeaders, "", op, spec, "sig1")

	// Hop-by-hop headers must be stripped before forwarding to backend
	if receivedConnection != "" {
		t.Errorf("Connection header should be stripped, got %q", receivedConnection)
	}
	if receivedKeepAlive != "" {
		t.Errorf("Keep-Alive header should be stripped, got %q", receivedKeepAlive)
	}
	// Non-hop-by-hop custom headers must pass through
	if receivedCustom != "kept" {
		t.Errorf("X-Custom should pass through, got %q", receivedCustom)
	}
}

// ---- recordResponse ----

func waitForRecord(store storage.Storage, opID string, timeout time.Duration) []*models.ResponseConfig {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		cfgs, _ := store.GetResponseConfigsByOperation(opID)
		if len(cfgs) > 0 {
			return cfgs
		}
		time.Sleep(5 * time.Millisecond)
	}
	return nil
}

func TestRecordResponse_CreatesNewEntry(t *testing.T) {
	backend := startFakeBackend(201, `{"id":"new"}`, map[string]string{"Content-Type": "application/json"})
	defer backend.Close()

	rec, store := makeTestRecorder(t)

	op := &models.Operation{ID: "op-record-1"}
	spec := &models.Spec{BackendURI: backend.URL}

	_, _, _, err := rec.ProxyAndRecord("POST", "/items", "", http.Header{}, `{"name":"x"}`, op, spec, "deadsig1")
	if err != nil {
		t.Fatalf("ProxyAndRecord: %v", err)
	}

	cfgs := waitForRecord(store, "op-record-1", 500*time.Millisecond)
	if len(cfgs) == 0 {
		t.Fatal("expected a recorded ResponseConfig to be created")
	}

	cfg := cfgs[0]
	if cfg.StatusCode != 201 {
		t.Errorf("expected status 201, got %d", cfg.StatusCode)
	}
	if cfg.Body != `{"id":"new"}` {
		t.Errorf("unexpected body: %q", cfg.Body)
	}
	if !cfg.Recorded {
		t.Error("expected Recorded flag to be true")
	}

	// Verify signature condition
	found := false
	for _, cond := range cfg.Conditions {
		if cond.Source == models.SourceSignature && cond.Value == "deadsig1" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected signature condition with value 'deadsig1', got %+v", cfg.Conditions)
	}
}

func TestRecordResponse_UpdatesExistingEntry(t *testing.T) {
	backend := startFakeBackend(200, `{"v":1}`, nil)
	defer backend.Close()

	rec, store := makeTestRecorder(t)
	op := &models.Operation{ID: "op-update-1"}
	spec := &models.Spec{BackendURI: backend.URL}

	// First call — creates entry
	rec.ProxyAndRecord("GET", "/x", "", http.Header{}, "", op, spec, "stablesig")
	cfgs := waitForRecord(store, "op-update-1", 500*time.Millisecond)
	if len(cfgs) == 0 {
		t.Fatal("first call should have created a recorded config")
	}
	firstID := cfgs[0].ID

	// Swap to backend returning v:2
	backend2 := startFakeBackend(200, `{"v":2}`, nil)
	defer backend2.Close()
	spec.BackendURI = backend2.URL

	// Second call with same signature — should update, not create
	rec.ProxyAndRecord("GET", "/x", "", http.Header{}, "", op, spec, "stablesig")
	time.Sleep(100 * time.Millisecond)

	cfgs2, _ := store.GetResponseConfigsByOperation("op-update-1")
	if len(cfgs2) != 1 {
		t.Errorf("expected exactly 1 recorded config after update, got %d", len(cfgs2))
	}
	if cfgs2[0].ID != firstID {
		t.Error("ID should not change on update")
	}
	if cfgs2[0].Body != `{"v":2}` {
		t.Errorf("body should be updated to v:2, got %q", cfgs2[0].Body)
	}
}

func TestSaveResponse_ConcurrentSameSignatureDoesNotDuplicate(t *testing.T) {
	inner := storage.NewMemoryStorage()
	store := newBarrierStorage(inner)
	rec := NewRecorder(store)
	op := &models.Operation{ID: "op-concurrent-1"}

	start := make(chan struct{})
	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		body := fmt.Sprintf(`{"version":%d}`, i+1)
		go func(body string) {
			defer wg.Done()
			<-start
			rec.SaveResponse(op, "sharedsig", 200, map[string]string{"Content-Type": "application/json"}, body, models.ResponseOriginProxy)
		}(body)
	}

	close(start)
	wg.Wait()

	cfgs, err := inner.GetResponseConfigsByOperation(op.ID)
	if err != nil {
		t.Fatalf("GetResponseConfigsByOperation: %v", err)
	}
	if len(cfgs) != 1 {
		t.Fatalf("expected exactly 1 recorded config after concurrent save, got %d", len(cfgs))
	}
	if cfgs[0].EffectiveOrigin() != models.ResponseOriginProxy {
		t.Fatalf("expected proxy origin, got %q", cfgs[0].EffectiveOrigin())
	}
}

func TestRecordResponse_NameContainsTimestamp(t *testing.T) {
	backend := startFakeBackend(200, "ok", nil)
	defer backend.Close()

	rec, store := makeTestRecorder(t)
	op := &models.Operation{ID: "op-name-1"}
	spec := &models.Spec{BackendURI: backend.URL}

	rec.ProxyAndRecord("GET", "/", "", http.Header{}, "", op, spec, "namesig")
	cfgs := waitForRecord(store, "op-name-1", 500*time.Millisecond)
	if len(cfgs) == 0 {
		t.Fatal("expected a recorded config")
	}

	name := cfgs[0].Name
	if len(name) == 0 {
		t.Error("expected a non-empty name")
	}
}

func TestGenerateRecordedID_Unique(t *testing.T) {
	ids := make(map[string]bool)
	for i := 0; i < 20; i++ {
		id := generateRecordedID()
		if ids[id] {
			t.Errorf("duplicate ID generated: %q", id)
		}
		ids[id] = true
	}
}

// ---- skipProxyRequestHeaders / skipRecordedResponseHeaders ----

func TestSkipProxyRequestHeaders_ContainsExpected(t *testing.T) {
	mustBeSkipped := []string{
		"Host", "Accept-Encoding", "Connection", "Transfer-Encoding",
		"Upgrade", "Keep-Alive", "Proxy-Connection",
	}
	for _, h := range mustBeSkipped {
		if !skipProxyRequestHeaders[http.CanonicalHeaderKey(h)] {
			t.Errorf("expected %q to be in skipProxyRequestHeaders", h)
		}
	}
}

func TestSkipRecordedResponseHeaders_ContainsExpected(t *testing.T) {
	mustBeSkipped := []string{
		"Content-Length", "Transfer-Encoding", "Content-Encoding",
		"Date", "Age", "Via", "Alt-Svc", "Trailer",
	}
	for _, h := range mustBeSkipped {
		if !skipRecordedResponseHeaders[http.CanonicalHeaderKey(h)] {
			t.Errorf("expected %q to be in skipRecordedResponseHeaders", h)
		}
	}
}

// ---- NewRecorder ----

func TestNewRecorder(t *testing.T) {
	store := storage.NewMemoryStorage()
	rec := NewRecorder(store)
	if rec == nil {
		t.Fatal("expected non-nil recorder")
	}
	if rec.httpClient == nil {
		t.Fatal("expected non-nil http client")
	}
}

// ---- JSON body forwarding ----

func TestProxyAndRecord_ForwardsRequestBody(t *testing.T) {
	var receivedBody string
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var m map[string]interface{}
		json.NewDecoder(r.Body).Decode(&m)
		data, _ := json.Marshal(m)
		receivedBody = string(data)
		w.WriteHeader(200)
	}))
	defer backend.Close()

	rec, _ := makeTestRecorder(t)
	spec := &models.Spec{BackendURI: backend.URL}
	op := &models.Operation{ID: "op-fwd"}

	rec.ProxyAndRecord("POST", "/", "", http.Header{}, `{"key":"value"}`, op, spec, "s1")
	time.Sleep(50 * time.Millisecond)

	if receivedBody != `{"key":"value"}` {
		t.Errorf("expected request body to be forwarded, got %q", receivedBody)
	}
}
