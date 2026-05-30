package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/prasenjit/go-virtual/internal/logging"
)

const (
	openAIProviderName      = "openai"
	claudeProviderName      = "claude"
	copilotProviderName     = "copilot"
	defaultOpenAIModel      = "gpt-4o-mini"
	defaultOpenAIBaseURL    = "https://api.openai.com/v1"
	defaultClaudeModel      = "claude-sonnet-4-6"
	defaultClaudeEndpoint   = "https://api.anthropic.com/v1/messages"
	defaultClaudeAPIVersion = "2023-06-01"
	defaultClaudeMaxTokens  = 4096
	defaultCopilotModel           = "gpt-4o"
	defaultCopilotBaseURL         = "https://api.githubcopilot.com"
	defaultCopilotTokenURL        = "https://api.github.com/copilot_internal/v2/token"
	defaultCopilotEditorVersion   = "vscode/1.96.0"
	defaultCopilotPluginVersion   = "copilot/1.155.0"
	defaultCopilotIntegrationID   = "vscode-chat"
	defaultCopilotOpenAIIntent    = "conversation-panel"
	copilotTokenRefreshBuffer     = 60 * time.Second
	defaultModel            = defaultOpenAIModel
)

// Config holds the AI generator configuration.
type Config struct {
	Provider string

	// HTTPProxy is an optional proxy URL (http/https/socks5) applied to all
	// outbound AI provider requests. Credentials can be embedded:
	// "http://user:pass@proxy.corp:8080"
	HTTPProxy string

	// Legacy OpenAI aliases kept for existing call sites and tests.
	APIKey   string
	Model    string
	Endpoint string

	OpenAI  ProviderConfig
	Claude  ClaudeProviderConfig
	Copilot CopilotProviderConfig
}

// ProviderConfig holds settings for OpenAI-compatible providers.
type ProviderConfig struct {
	APIKey   string
	Model    string
	BaseURL  string
	Endpoint string
}

// ClaudeProviderConfig holds settings for Anthropic Claude.
type ClaudeProviderConfig struct {
	APIKey     string
	Model      string
	BaseURL    string
	Endpoint   string
	APIVersion string
}

// Status reports the selected AI provider and whether it is configured.
type Status struct {
	Configured bool   `json:"configured"`
	Provider   string `json:"provider"`
	Model      string `json:"model,omitempty"`
}

type providerRequest struct {
	SystemPrompt string
	Messages     []ChatMessage
	Temperature  float64
}

type completionProvider interface {
	Name() string
	DisplayName() string
	Model() string
	IsConfigured() bool
	MissingConfigMessage() string
	Complete(ctx context.Context, req providerRequest) (string, error)
}

// CopilotProviderConfig holds settings for GitHub Copilot.
type CopilotProviderConfig struct {
	// OAuthToken is the GitHub OAuth token (gho_...) used to exchange for a
	// short-lived Copilot API token. Copy from ~/.config/github-copilot/apps.json.
	OAuthToken string
	// Model is the model used for completions. Defaults to "gpt-4o".
	Model string
	// BaseURL overrides the Copilot completions base URL.
	BaseURL string
	// TokenURL overrides the Copilot token exchange endpoint.
	TokenURL string
	// EditorVersion is sent as the editor-version header on all Copilot requests.
	// Defaults to "vscode/1.96.0".
	EditorVersion string
	// EditorPluginVersion is sent as the editor-plugin-version header on all Copilot requests.
	// Defaults to "copilot/1.155.0".
	EditorPluginVersion string
	// IntegrationID is sent as the copilot-integration-id header on all Copilot requests.
	// Defaults to "vscode-chat".
	IntegrationID string
	// OpenAIIntent is sent as the openai-intent header on token-exchange and
	// completion requests. Defaults to "conversation-panel".
	OpenAIIntent string
}

type openAIProvider struct {
	cfg      ProviderConfig
	client   *http.Client
	endpoint string
}

type claudeProvider struct {
	cfg      ClaudeProviderConfig
	client   *http.Client
	endpoint string
}

type copilotProvider struct {
	cfg      CopilotProviderConfig
	client   *http.Client
	endpoint string
	tokenURL string

	mu        sync.Mutex
	cachedTok string
	expiresAt time.Time
}

type invalidProvider struct {
	name string
}

func (cfg Config) Normalize() Config {
	cfg.Provider = strings.ToLower(strings.TrimSpace(cfg.Provider))
	if cfg.Provider == "" {
		cfg.Provider = openAIProviderName
	}

	if strings.TrimSpace(cfg.OpenAI.APIKey) == "" {
		cfg.OpenAI.APIKey = strings.TrimSpace(cfg.APIKey)
	}
	if strings.TrimSpace(cfg.OpenAI.Model) == "" {
		cfg.OpenAI.Model = strings.TrimSpace(cfg.Model)
	}
	if strings.TrimSpace(cfg.OpenAI.Endpoint) == "" {
		cfg.OpenAI.Endpoint = strings.TrimSpace(cfg.Endpoint)
	}
	if strings.TrimSpace(cfg.OpenAI.Model) == "" {
		cfg.OpenAI.Model = defaultOpenAIModel
	}
	if strings.TrimSpace(cfg.Model) == "" {
		cfg.Model = cfg.OpenAI.Model
	}
	if strings.TrimSpace(cfg.APIKey) == "" {
		cfg.APIKey = cfg.OpenAI.APIKey
	}
	if strings.TrimSpace(cfg.Endpoint) == "" {
		cfg.Endpoint = cfg.OpenAI.Endpoint
	}

	if strings.TrimSpace(cfg.Claude.Model) == "" {
		cfg.Claude.Model = defaultClaudeModel
	}
	if strings.TrimSpace(cfg.Claude.APIVersion) == "" {
		cfg.Claude.APIVersion = defaultClaudeAPIVersion
	}

	if strings.TrimSpace(cfg.Copilot.Model) == "" {
		cfg.Copilot.Model = defaultCopilotModel
	}
	if strings.TrimSpace(cfg.Copilot.EditorVersion) == "" {
		cfg.Copilot.EditorVersion = defaultCopilotEditorVersion
	}
	if strings.TrimSpace(cfg.Copilot.EditorPluginVersion) == "" {
		cfg.Copilot.EditorPluginVersion = defaultCopilotPluginVersion
	}
	if strings.TrimSpace(cfg.Copilot.IntegrationID) == "" {
		cfg.Copilot.IntegrationID = defaultCopilotIntegrationID
	}
	if strings.TrimSpace(cfg.Copilot.OpenAIIntent) == "" {
		cfg.Copilot.OpenAIIntent = defaultCopilotOpenAIIntent
	}

	return cfg
}

func newCompletionProvider(cfg Config, client *http.Client) completionProvider {
	cfg = cfg.Normalize()

	switch cfg.Provider {
	case "", openAIProviderName:
		return &openAIProvider{
			cfg:      cfg.OpenAI,
			client:   client,
			endpoint: openAIEndpoint(cfg.OpenAI),
		}
	case claudeProviderName:
		return &claudeProvider{
			cfg:      cfg.Claude,
			client:   client,
			endpoint: claudeEndpoint(cfg.Claude),
		}
	case copilotProviderName:
		return &copilotProvider{
			cfg:      cfg.Copilot,
			client:   client,
			endpoint: copilotEndpoint(cfg.Copilot),
			tokenURL: copilotTokenURL(cfg.Copilot),
		}
	default:
		return &invalidProvider{name: cfg.Provider}
	}
}

func openAIEndpoint(cfg ProviderConfig) string {
	if endpoint := strings.TrimSpace(cfg.Endpoint); endpoint != "" {
		return endpoint
	}
	baseURL := strings.TrimSpace(cfg.BaseURL)
	if baseURL == "" {
		baseURL = defaultOpenAIBaseURL
	}
	if strings.HasSuffix(baseURL, "/chat/completions") {
		return baseURL
	}
	return strings.TrimRight(baseURL, "/") + "/chat/completions"
}

func claudeEndpoint(cfg ClaudeProviderConfig) string {
	if endpoint := strings.TrimSpace(cfg.Endpoint); endpoint != "" {
		return endpoint
	}
	baseURL := strings.TrimSpace(cfg.BaseURL)
	if baseURL == "" {
		return defaultClaudeEndpoint
	}
	if strings.HasSuffix(baseURL, "/messages") {
		return baseURL
	}
	return strings.TrimRight(baseURL, "/") + "/messages"
}

func (p *openAIProvider) Name() string        { return openAIProviderName }
func (p *openAIProvider) DisplayName() string { return "OpenAI" }
func (p *openAIProvider) Model() string       { return p.cfg.Model }
func (p *openAIProvider) IsConfigured() bool  { return strings.TrimSpace(p.cfg.APIKey) != "" }

func (p *openAIProvider) MissingConfigMessage() string {
	return "AI generation is not configured — set ai.openai.apiKey (or legacy ai.openaiApiKey / GOVIRTUAL_AI_OPENAIAPIKEY) for the selected OpenAI provider"
}

func (p *openAIProvider) Complete(ctx context.Context, req providerRequest) (string, error) {
	logger := logging.Logger("ai.provider").With("provider", openAIProviderName, "model", p.cfg.Model)
	logger.Debug("Starting OpenAI completion request",
		"event", "ai_provider_request_started",
		"endpoint", p.endpoint,
		"message_count", len(req.Messages),
		"has_system_prompt", strings.TrimSpace(req.SystemPrompt) != "",
	)
	reqBody := map[string]any{
		"model":           p.cfg.Model,
		"messages":        buildOpenAIMessages(req),
		"temperature":     req.Temperature,
		"response_format": map[string]string{"type": "json_object"},
	}
	start := time.Now()
	content, err := doOpenAIRequest(ctx, p.client, p.endpoint, p.cfg.APIKey, reqBody)
	if err != nil {
		logger.Error("OpenAI completion request failed",
			"event", "ai_provider_request_failed",
			"endpoint", p.endpoint,
			"duration_ms", time.Since(start).Milliseconds(),
			"error", err,
		)
		return "", err
	}
	logger.Info("OpenAI completion request succeeded",
		"event", "ai_provider_request_succeeded",
		"endpoint", p.endpoint,
		"duration_ms", time.Since(start).Milliseconds(),
	)
	return content, nil
}

func buildOpenAIMessages(req providerRequest) []map[string]string {
	messages := make([]map[string]string, 0, len(req.Messages)+1)
	if strings.TrimSpace(req.SystemPrompt) != "" {
		messages = append(messages, map[string]string{"role": "system", "content": req.SystemPrompt})
	}
	for _, msg := range req.Messages {
		messages = append(messages, map[string]string{"role": msg.Role, "content": msg.Content})
	}
	return messages
}

func doOpenAIRequest(ctx context.Context, client *http.Client, endpoint, apiKey string, reqBody map[string]any) (string, error) {
	data, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(data))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("OpenAI request failed: %w", err)
	}
	defer resp.Body.Close()

	var apiResp struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return "", fmt.Errorf("failed to decode OpenAI response: %w", err)
	}
	if apiResp.Error != nil {
		return "", fmt.Errorf("OpenAI error: %s", apiResp.Error.Message)
	}
	if len(apiResp.Choices) == 0 {
		return "", fmt.Errorf("OpenAI returned no choices")
	}
	return apiResp.Choices[0].Message.Content, nil
}

func (p *claudeProvider) Name() string        { return claudeProviderName }
func (p *claudeProvider) DisplayName() string { return "Claude" }
func (p *claudeProvider) Model() string       { return p.cfg.Model }
func (p *claudeProvider) IsConfigured() bool  { return strings.TrimSpace(p.cfg.APIKey) != "" }

func (p *claudeProvider) MissingConfigMessage() string {
	return "AI generation is not configured — set ai.claude.apiKey for the selected Claude provider"
}

func (p *claudeProvider) Complete(ctx context.Context, req providerRequest) (string, error) {
	logger := logging.Logger("ai.provider").With("provider", claudeProviderName, "model", p.cfg.Model)
	logger.Debug("Starting Claude completion request",
		"event", "ai_provider_request_started",
		"endpoint", p.endpoint,
		"message_count", len(req.Messages),
		"has_system_prompt", strings.TrimSpace(req.SystemPrompt) != "",
	)
	reqBody := map[string]any{
		"model":       p.cfg.Model,
		"system":      req.SystemPrompt,
		"messages":    buildClaudeMessages(req.Messages),
		"temperature": req.Temperature,
		"max_tokens":  defaultClaudeMaxTokens,
	}
	data, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.endpoint, bytes.NewReader(data))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", p.cfg.APIKey)
	httpReq.Header.Set("anthropic-version", p.cfg.APIVersion)

	start := time.Now()
	resp, err := p.client.Do(httpReq)
	if err != nil {
		wrapped := fmt.Errorf("Claude request failed: %w", err)
		logger.Error("Claude completion request failed",
			"event", "ai_provider_request_failed",
			"endpoint", p.endpoint,
			"duration_ms", time.Since(start).Milliseconds(),
			"error", wrapped,
		)
		return "", wrapped
	}
	defer resp.Body.Close()

	var apiResp struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
		Error *struct {
			Type    string `json:"type"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		wrapped := fmt.Errorf("failed to decode Claude response: %w", err)
		logger.Error("Claude completion response decode failed",
			"event", "ai_provider_response_decode_failed",
			"endpoint", p.endpoint,
			"http_status", resp.StatusCode,
			"duration_ms", time.Since(start).Milliseconds(),
			"error", wrapped,
		)
		return "", wrapped
	}
	if apiResp.Error != nil {
		msg := strings.TrimSpace(apiResp.Error.Message)
		if strings.Contains(strings.ToLower(msg), "model") {
			msg = fmt.Sprintf("%s (check ai.claude.model; current value: %s)", msg, p.cfg.Model)
		}
		if apiResp.Error.Type != "" {
			wrapped := fmt.Errorf("Claude error (%s, HTTP %d): %s", apiResp.Error.Type, resp.StatusCode, msg)
			logger.Error("Claude completion request returned API error",
				"event", "ai_provider_api_error",
				"endpoint", p.endpoint,
				"http_status", resp.StatusCode,
				"error_type", apiResp.Error.Type,
				"duration_ms", time.Since(start).Milliseconds(),
				"error", wrapped,
			)
			return "", wrapped
		}
		wrapped := fmt.Errorf("Claude error (HTTP %d): %s", resp.StatusCode, msg)
		logger.Error("Claude completion request returned API error",
			"event", "ai_provider_api_error",
			"endpoint", p.endpoint,
			"http_status", resp.StatusCode,
			"duration_ms", time.Since(start).Milliseconds(),
			"error", wrapped,
		)
		return "", wrapped
	}
	for _, block := range apiResp.Content {
		if block.Type == "text" && strings.TrimSpace(block.Text) != "" {
			logger.Info("Claude completion request succeeded",
				"event", "ai_provider_request_succeeded",
				"endpoint", p.endpoint,
				"http_status", resp.StatusCode,
				"duration_ms", time.Since(start).Milliseconds(),
			)
			return block.Text, nil
		}
	}
	wrapped := fmt.Errorf("Claude returned no text content")
	logger.Error("Claude completion response contained no text content",
		"event", "ai_provider_response_empty",
		"endpoint", p.endpoint,
		"http_status", resp.StatusCode,
		"duration_ms", time.Since(start).Milliseconds(),
		"error", wrapped,
	)
	return "", wrapped
}

func buildClaudeMessages(messages []ChatMessage) []map[string]any {
	result := make([]map[string]any, 0, len(messages))
	for _, msg := range messages {
		result = append(result, map[string]any{
			"role": msg.Role,
			"content": []map[string]string{
				{"type": "text", "text": msg.Content},
			},
		})
	}
	return result
}

func (p *invalidProvider) Name() string {
	if p.name == "" {
		return "unknown"
	}
	return p.name
}

func (p *invalidProvider) DisplayName() string {
	return p.Name()
}

func (p *invalidProvider) Model() string      { return "" }
func (p *invalidProvider) IsConfigured() bool { return false }

func (p *invalidProvider) MissingConfigMessage() string {
	return fmt.Sprintf("AI generation is not configured — provider %q is not supported; use %q, %q, or %q", p.Name(), openAIProviderName, claudeProviderName, copilotProviderName)
}

func (p *invalidProvider) Complete(_ context.Context, _ providerRequest) (string, error) {
	return "", fmt.Errorf("AI provider %q is not supported", p.Name())
}

func newHTTPClient(proxyURL string) *http.Client {
	transport := &http.Transport{}
	if proxyURL != "" {
		if parsed, err := url.Parse(proxyURL); err == nil {
			transport.Proxy = http.ProxyURL(parsed)
		}
	}
	return &http.Client{Timeout: 30 * time.Second, Transport: transport}
}

func copilotEndpoint(cfg CopilotProviderConfig) string {
	base := strings.TrimSpace(cfg.BaseURL)
	if base == "" {
		base = defaultCopilotBaseURL
	}
	return strings.TrimRight(base, "/") + "/chat/completions"
}

func copilotTokenURL(cfg CopilotProviderConfig) string {
	if u := strings.TrimSpace(cfg.TokenURL); u != "" {
		return u
	}
	return defaultCopilotTokenURL
}

func (p *copilotProvider) Name() string        { return copilotProviderName }
func (p *copilotProvider) DisplayName() string { return "GitHub Copilot" }
func (p *copilotProvider) Model() string       { return p.cfg.Model }
func (p *copilotProvider) IsConfigured() bool  { return strings.TrimSpace(p.cfg.OAuthToken) != "" }

func (p *copilotProvider) MissingConfigMessage() string {
	return `AI generation is not configured — set ai.copilot.oauthToken to the oauth_token value from ~/.config/github-copilot/apps.json for the selected Copilot provider`
}

// getToken returns a valid short-lived Copilot API token, exchanging or
// refreshing via the GitHub token endpoint as needed.
func (p *copilotProvider) getToken(ctx context.Context) (string, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.cachedTok != "" && time.Now().Add(copilotTokenRefreshBuffer).Before(p.expiresAt) {
		return p.cachedTok, nil
	}

	tok, expiresAt, err := exchangeCopilotToken(ctx, p.client, p.tokenURL, p.cfg.OAuthToken, p.copilotHeaders())
	if err != nil {
		return "", err
	}
	p.cachedTok = tok
	p.expiresAt = expiresAt
	return tok, nil
}

// copilotHeaders returns the identity headers sent on every Copilot request.
func (p *copilotProvider) copilotHeaders() map[string]string {
	return map[string]string{
		"editor-version":        p.cfg.EditorVersion,
		"editor-plugin-version": p.cfg.EditorPluginVersion,
		"copilot-integration-id": p.cfg.IntegrationID,
		"openai-intent":         p.cfg.OpenAIIntent,
	}
}

func exchangeCopilotToken(ctx context.Context, client *http.Client, tokenURL, oauthToken string, headers map[string]string) (string, time.Time, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, tokenURL, nil)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("failed to create Copilot token request: %w", err)
	}
	req.Header.Set("Authorization", "token "+oauthToken)
	req.Header.Set("Accept", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("Copilot token exchange failed: %w", err)
	}
	defer resp.Body.Close()

	var body struct {
		Token     string      `json:"token"`
		ExpiresAt json.Number `json:"expires_at"` // API returns Unix timestamp (int) not RFC3339 string
		ErrorType string      `json:"error_type"`
		Message   string      `json:"message"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return "", time.Time{}, fmt.Errorf("failed to decode Copilot token response: %w", err)
	}
	if resp.StatusCode != http.StatusOK || body.Token == "" {
		msg := body.Message
		if msg == "" {
			msg = fmt.Sprintf("HTTP %d", resp.StatusCode)
		}
		return "", time.Time{}, fmt.Errorf("Copilot token exchange error: %s", msg)
	}

	expiresAt := time.Now().Add(30 * time.Minute) // safe default
	if s := body.ExpiresAt.String(); s != "" {
		// Try Unix timestamp (int) first — what the real API returns.
		if unix, err := body.ExpiresAt.Int64(); err == nil {
			expiresAt = time.Unix(unix, 0)
		} else if t, err := time.Parse(time.RFC3339, s); err == nil {
			// Fallback: accept RFC3339 string for compatibility with mocks.
			expiresAt = t
		}
	}
	return body.Token, expiresAt, nil
}

func (p *copilotProvider) Complete(ctx context.Context, req providerRequest) (string, error) {
	logger := logging.Logger("ai.provider").With("provider", copilotProviderName, "model", p.cfg.Model)
	logger.Debug("Starting Copilot completion request",
		"event", "ai_provider_request_started",
		"endpoint", p.endpoint,
		"message_count", len(req.Messages),
		"has_system_prompt", strings.TrimSpace(req.SystemPrompt) != "",
	)

	tok, err := p.getToken(ctx)
	if err != nil {
		logger.Error("Failed to obtain Copilot token",
			"event", "ai_provider_token_failed",
			"error", err,
		)
		return "", err
	}

	reqBody := map[string]any{
		"model":           p.cfg.Model,
		"messages":        buildOpenAIMessages(req),
		"temperature":     req.Temperature,
		"response_format": map[string]string{"type": "json_object"},
	}

	// Clone the request-building logic from doOpenAIRequest so we can inject
	// the extra Copilot editor-identity headers.
	data, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal Copilot request: %w", err)
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.endpoint, bytes.NewReader(data))
	if err != nil {
		return "", fmt.Errorf("failed to create Copilot request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+tok)
	for k, v := range p.copilotHeaders() {
		httpReq.Header.Set(k, v)
	}

	start := time.Now()
	resp, err := p.client.Do(httpReq)
	if err != nil {
		wrapped := fmt.Errorf("Copilot request failed: %w", err)
		logger.Error("Copilot completion request failed",
			"event", "ai_provider_request_failed",
			"endpoint", p.endpoint,
			"duration_ms", time.Since(start).Milliseconds(),
			"error", wrapped,
		)
		return "", wrapped
	}
	defer resp.Body.Close()

	var apiResp struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return "", fmt.Errorf("failed to decode Copilot response: %w", err)
	}
	if apiResp.Error != nil {
		wrapped := fmt.Errorf("Copilot error (HTTP %d): %s", resp.StatusCode, apiResp.Error.Message)
		logger.Error("Copilot completion request returned API error",
			"event", "ai_provider_api_error",
			"endpoint", p.endpoint,
			"http_status", resp.StatusCode,
			"duration_ms", time.Since(start).Milliseconds(),
			"error", wrapped,
		)
		return "", wrapped
	}
	if len(apiResp.Choices) == 0 {
		return "", fmt.Errorf("Copilot returned no choices")
	}
	logger.Info("Copilot completion request succeeded",
		"event", "ai_provider_request_succeeded",
		"endpoint", p.endpoint,
		"duration_ms", time.Since(start).Milliseconds(),
	)
	return apiResp.Choices[0].Message.Content, nil
}

func titleProvider(name string) string {
	switch name {
	case openAIProviderName:
		return "OpenAI"
	case claudeProviderName:
		return "Claude"
	case copilotProviderName:
		return "GitHub Copilot"
	default:
		if name == "" {
			return "AI"
		}
		return strings.ToUpper(name[:1]) + name[1:]
	}
}
