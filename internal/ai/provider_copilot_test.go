package ai

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

// mockCopilotTokenServer starts a test server simulating the Copilot token
// exchange endpoint. It records how many times it was called.
func mockCopilotTokenServer(t *testing.T, statusCode int, token, expiresAt string) (*httptest.Server, *atomic.Int32) {
	t.Helper()
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		if statusCode != http.StatusOK {
			w.WriteHeader(statusCode)
			json.NewEncoder(w).Encode(map[string]string{"message": "unauthorized"})
			return
		}
		json.NewEncoder(w).Encode(map[string]string{
			"token":      token,
			"expires_at": expiresAt,
		})
	}))
	t.Cleanup(srv.Close)
	return srv, &calls
}

// mockCopilotCompletionsServer starts a test server simulating the Copilot
// chat completions endpoint. It verifies the required Copilot headers are present.
func mockCopilotCompletionsServer(t *testing.T, content string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("editor-version") == "" {
			http.Error(w, "missing editor-version", http.StatusBadRequest)
			return
		}
		if r.Header.Get("editor-plugin-version") == "" {
			http.Error(w, "missing editor-plugin-version", http.StatusBadRequest)
			return
		}
		if r.Header.Get("openai-intent") == "" {
			http.Error(w, "missing openai-intent", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{
				{"message": map[string]string{"content": content}},
			},
		})
	}))
	t.Cleanup(srv.Close)
	return srv
}

func newCopilotProviderForTest(t *testing.T, oauthToken, tokenURL, completionsURL string) *copilotProvider {
	t.Helper()
	return &copilotProvider{
		cfg: CopilotProviderConfig{
			OAuthToken: oauthToken,
			Model:      "gpt-4o",
		},
		client:   &http.Client{Timeout: 5 * time.Second},
		endpoint: completionsURL + "/chat/completions",
		tokenURL: tokenURL,
	}
}

func TestCopilotProvider_IsConfigured(t *testing.T) {
	configured := &copilotProvider{cfg: CopilotProviderConfig{OAuthToken: "gho_abc"}}
	if !configured.IsConfigured() {
		t.Fatal("expected IsConfigured() = true when OAuthToken is set")
	}

	unconfigured := &copilotProvider{cfg: CopilotProviderConfig{}}
	if unconfigured.IsConfigured() {
		t.Fatal("expected IsConfigured() = false when OAuthToken is empty")
	}
}

func TestCopilotProvider_Complete_Success(t *testing.T) {
	expiry := time.Now().Add(30 * time.Minute).UTC().Format(time.RFC3339)
	tokSrv, calls := mockCopilotTokenServer(t, http.StatusOK, "test-copilot-tok", expiry)
	compSrv := mockCopilotCompletionsServer(t, `{"status":200}`)

	p := newCopilotProviderForTest(t, "gho_test", tokSrv.URL, compSrv.URL)

	result, err := p.Complete(context.Background(), providerRequest{
		SystemPrompt: "You are a test assistant",
		Messages:     []ChatMessage{{Role: "user", Content: "hello"}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == "" {
		t.Fatal("expected non-empty result")
	}
	if calls.Load() != 1 {
		t.Fatalf("expected 1 token exchange call, got %d", calls.Load())
	}
}

func TestCopilotProvider_TokenCaching(t *testing.T) {
	expiry := time.Now().Add(30 * time.Minute).UTC().Format(time.RFC3339)
	tokSrv, calls := mockCopilotTokenServer(t, http.StatusOK, "test-copilot-tok", expiry)
	compSrv := mockCopilotCompletionsServer(t, `{"status":200}`)

	p := newCopilotProviderForTest(t, "gho_test", tokSrv.URL, compSrv.URL)

	req := providerRequest{Messages: []ChatMessage{{Role: "user", Content: "hi"}}}

	if _, err := p.Complete(context.Background(), req); err != nil {
		t.Fatalf("first call failed: %v", err)
	}
	if _, err := p.Complete(context.Background(), req); err != nil {
		t.Fatalf("second call failed: %v", err)
	}

	// Token should be reused; exchange endpoint called only once.
	if calls.Load() != 1 {
		t.Fatalf("expected 1 token exchange call (cached), got %d", calls.Load())
	}
}

func TestCopilotProvider_TokenRefreshOnExpiry(t *testing.T) {
	expiry := time.Now().Add(30 * time.Minute).UTC().Format(time.RFC3339)
	tokSrv, calls := mockCopilotTokenServer(t, http.StatusOK, "test-copilot-tok", expiry)
	compSrv := mockCopilotCompletionsServer(t, `{"status":200}`)

	p := newCopilotProviderForTest(t, "gho_test", tokSrv.URL, compSrv.URL)

	// Seed the cache with an already-expired token.
	p.cachedTok = "old-tok"
	p.expiresAt = time.Now().Add(-1 * time.Minute)

	req := providerRequest{Messages: []ChatMessage{{Role: "user", Content: "hi"}}}
	if _, err := p.Complete(context.Background(), req); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Token was expired, so exchange should have been called.
	if calls.Load() != 1 {
		t.Fatalf("expected 1 token exchange call after expiry, got %d", calls.Load())
	}
	if p.cachedTok == "old-tok" {
		t.Fatal("expected cached token to be updated after refresh")
	}
}

func TestCopilotProvider_FailedTokenExchange(t *testing.T) {
	tokSrv, _ := mockCopilotTokenServer(t, http.StatusUnauthorized, "", "")
	compSrv := mockCopilotCompletionsServer(t, `{}`)

	p := newCopilotProviderForTest(t, "gho_bad", tokSrv.URL, compSrv.URL)

	_, err := p.Complete(context.Background(), providerRequest{
		Messages: []ChatMessage{{Role: "user", Content: "hi"}},
	})
	if err == nil {
		t.Fatal("expected error on failed token exchange")
	}
}

func TestCopilotProvider_MissingOAuthToken(t *testing.T) {
	p := &copilotProvider{
		cfg: CopilotProviderConfig{OAuthToken: ""},
	}
	if p.IsConfigured() {
		t.Fatal("expected IsConfigured() = false with empty OAuthToken")
	}
}

func TestNewHTTPClient_WithProxy(t *testing.T) {
	client := newHTTPClient("http://user:pass@proxy.corp:8080")
	if client == nil {
		t.Fatal("expected non-nil http.Client")
	}
	transport, ok := client.Transport.(*http.Transport)
	if !ok {
		t.Fatal("expected *http.Transport")
	}
	if transport.Proxy == nil {
		t.Fatal("expected Proxy to be set")
	}
}

func TestNewHTTPClient_WithoutProxy(t *testing.T) {
	client := newHTTPClient("")
	if client == nil {
		t.Fatal("expected non-nil http.Client")
	}
	transport, ok := client.Transport.(*http.Transport)
	if !ok {
		t.Fatal("expected *http.Transport")
	}
	if transport.Proxy != nil {
		t.Fatal("expected Proxy to be nil when no proxy URL is given")
	}
}
