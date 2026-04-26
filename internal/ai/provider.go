package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/prasenjit/go-virtual/internal/logging"
)

const (
	openAIProviderName      = "openai"
	claudeProviderName      = "claude"
	defaultOpenAIModel      = "gpt-4o-mini"
	defaultOpenAIBaseURL    = "https://api.openai.com/v1"
	defaultClaudeModel      = "claude-sonnet-4-6"
	defaultClaudeEndpoint   = "https://api.anthropic.com/v1/messages"
	defaultClaudeAPIVersion = "2023-06-01"
	defaultClaudeMaxTokens  = 4096
	defaultModel            = defaultOpenAIModel
)

// Config holds the AI generator configuration.
type Config struct {
	Provider string

	// Legacy OpenAI aliases kept for existing call sites and tests.
	APIKey   string
	Model    string
	Endpoint string

	OpenAI ProviderConfig
	Claude ClaudeProviderConfig
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
	return fmt.Sprintf("AI generation is not configured — provider %q is not supported; use %q or %q", p.Name(), openAIProviderName, claudeProviderName)
}

func (p *invalidProvider) Complete(_ context.Context, _ providerRequest) (string, error) {
	return "", fmt.Errorf("AI provider %q is not supported", p.Name())
}

func newHTTPClient() *http.Client {
	return &http.Client{Timeout: 30 * time.Second}
}

func titleProvider(name string) string {
	switch name {
	case openAIProviderName:
		return "OpenAI"
	case claudeProviderName:
		return "Claude"
	default:
		if name == "" {
			return "AI"
		}
		return strings.ToUpper(name[:1]) + name[1:]
	}
}
