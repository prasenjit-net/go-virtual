package proxy

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/prasenjit/go-virtual/internal/models"
	"github.com/prasenjit/go-virtual/internal/storage"
)

// skipProxyRequestHeaders lists headers that must NOT be forwarded to the
// backend in proxy mode.
//
//   - Hop-by-hop headers (Connection, Transfer-Encoding …) are transport-level
//     and must not cross proxy boundaries per RFC 7230 §6.1.
//   - Host: Go's http.Client derives it from the request URL; forwarding the
//     original value (e.g. "localhost:8000") would send the wrong Host to the
//     real backend.
//   - Accept-Encoding: if forwarded the backend may return a compressed body
//     that Go's http.Client will NOT auto-decompress (it only does so when it
//     added the header itself), producing garbled stored bytes.
var skipProxyRequestHeaders = map[string]bool{
	"Host":                true,
	"Accept-Encoding":     true,
	"Connection":          true,
	"Transfer-Encoding":   true,
	"Upgrade":             true,
	"Proxy-Connection":    true,
	"Keep-Alive":          true,
	"TE":                  true,
	"Trailers":            true,
	"Proxy-Authorization": true,
	"Proxy-Authenticate":  true,
}

// skipRecordedResponseHeaders lists headers that must NOT be stored when
// recording a backend response, nor set when replaying a recorded response.
//
//   - Content-Length / Transfer-Encoding: managed by Go's ResponseWriter;
//     pre-setting them causes IncompleteRead errors on the client side.
//   - Content-Encoding: http.Client already decompresses the body, so
//     re-advertising it would claim a compression that is no longer there.
//   - Date: stale on replay; Go's HTTP server injects the current date.
//   - Age: CDN/cache age counter, meaningless in a virtual context.
//   - Via: reveals the real backend proxy chain, misleading when serving virtual.
//   - Alt-Svc: tells clients to connect to the real backend via HTTP/3 or
//     alternative addresses, which is wrong for a virtual server.
//   - Trailer: HTTP chunked-encoding trailer metadata, irrelevant for stored
//     responses.
var skipRecordedResponseHeaders = map[string]bool{
	"Content-Length":    true,
	"Transfer-Encoding": true,
	"Content-Encoding":  true,
	"Date":              true,
	"Age":               true,
	"Via":               true,
	"Alt-Svc":           true,
	"Trailer":           true,
}

// Recorder proxies requests to an upstream backend and persists generated
// responses as replayable ResponseConfigs keyed by request signature.
type Recorder struct {
	store      storage.Storage
	httpClient *http.Client
}

// NewRecorder creates a new Recorder with a permissive TLS transport suitable
// for development environments.
func NewRecorder(store storage.Storage) *Recorder {
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, //nolint:gosec
	}
	return &Recorder{
		store: store,
		httpClient: &http.Client{
			Transport: transport,
			Timeout:   30 * time.Second,
		},
	}
}

// SetHTTPClient replaces the HTTP client used for backend requests.
// Call this after NewRecorder to configure mTLS or custom timeouts.
func (r *Recorder) SetHTTPClient(client *http.Client) {
	r.httpClient = client
}

// ProxyAndRecord forwards the original request to the backend identified by
// spec.BackendURI, records the response as a ResponseConfig for the operation
// (keyed by the pre-computed signature), and returns the backend response data
// so the engine can write it directly to the client.
//
// The method is intentionally fire-and-forget for the recording part: if
// persistence fails the response is still returned to the caller.
func (r *Recorder) ProxyAndRecord(
	method string,
	requestPath string,
	rawQuery string,
	reqHeaders http.Header,
	reqBody string,
	operation *models.Operation,
	spec *models.Spec,
	signature string,
) (statusCode int, respHeaders map[string]string, respBody string, err error) {
	// Build the backend URL by stripping the spec basePath from the request path
	backendPath := requestPath
	if spec.BasePath != "" && spec.BasePath != "/" {
		backendPath = strings.TrimPrefix(requestPath, spec.BasePath)
	}
	if backendPath == "" {
		backendPath = "/"
	}

	backendURL := strings.TrimRight(spec.BackendURI, "/") + backendPath
	if rawQuery != "" {
		backendURL += "?" + rawQuery
	}

	// Build the outgoing request
	var bodyReader io.Reader
	if reqBody != "" {
		bodyReader = bytes.NewBufferString(reqBody)
	}

	backendReq, err := http.NewRequest(method, backendURL, bodyReader)
	if err != nil {
		return 0, nil, "", fmt.Errorf("proxy: failed to build backend request: %w", err)
	}

	// Forward request headers, excluding any that must not cross the proxy
	// boundary (see skipProxyRequestHeaders).
	for key, vals := range reqHeaders {
		if skipProxyRequestHeaders[http.CanonicalHeaderKey(key)] {
			continue
		}
		for _, val := range vals {
			backendReq.Header.Add(key, val)
		}
	}

	// Execute backend request
	resp, err := r.httpClient.Do(backendReq)
	if err != nil {
		return 0, nil, "", fmt.Errorf("proxy: backend request failed: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, nil, "", fmt.Errorf("proxy: failed to read backend response body: %w", err)
	}

	respBody = string(bodyBytes)

	// Strip headers that must not be stored (see skipRecordedResponseHeaders).
	respHeaders = make(map[string]string)
	for key, vals := range resp.Header {
		if skipRecordedResponseHeaders[http.CanonicalHeaderKey(key)] {
			continue
		}
		if len(vals) > 0 {
			respHeaders[key] = vals[0]
		}
	}
	statusCode = resp.StatusCode

	// Record the response asynchronously so it doesn't block the response path.
	go r.SaveResponse(operation, signature, statusCode, respHeaders, respBody, models.ResponseOriginProxy)

	return statusCode, respHeaders, respBody, nil
}

// SaveResponse saves (or updates) a generated response as a replayable
// ResponseConfig for the given operation keyed by request signature and origin.
func (r *Recorder) SaveResponse(
	op *models.Operation,
	signature string,
	statusCode int,
	headers map[string]string,
	body string,
	origin string,
) {
	configs, err := r.store.GetResponseConfigsByOperation(op.ID)
	if err != nil {
		return
	}

	origin = models.NormalizeResponseOrigin(origin, true)

	// Check whether a generated response for this signature and origin already exists.
	var existing *models.ResponseConfig
	for _, cfg := range configs {
		if !cfg.Recorded || cfg.EffectiveOrigin() != origin {
			continue
		}
		for _, cond := range cfg.Conditions {
			if cond.Source == models.SourceSignature && cond.Value == signature {
				existing = cfg
				break
			}
		}
		if existing != nil {
			break
		}
	}

	if existing != nil {
		// Update the existing entry with the latest generated response.
		existing.StatusCode = statusCode
		existing.Headers = headers
		existing.Body = body
		_ = r.store.UpdateResponseConfig(existing)
		return
	}

	// Create a new generated ResponseConfig.
	now := time.Now()
	sigPreview := signature
	if len(sigPreview) > 8 {
		sigPreview = sigPreview[:8]
	}
	namePrefix := "[Recorded]"
	descriptionPrefix := "Auto-recorded from backend"
	if origin == models.ResponseOriginAI {
		namePrefix = "[AI]"
		descriptionPrefix = "Auto-generated by AI"
	}
	cfg := &models.ResponseConfig{
		ID:          generateRecordedID(),
		OperationID: op.ID,
		Name:        fmt.Sprintf("%s %s", namePrefix, now.Format("2006-01-02 15:04:05")),
		Description: fmt.Sprintf("%s at %s (sig: %s)", descriptionPrefix, now.Format(time.RFC3339), sigPreview),
		Tag:         models.DefaultTagName,
		Priority:    1, // Just after highest-priority manual responses (priority 0)
		Conditions: []models.Condition{
			{
				Source:   models.SourceSignature,
				Key:      "",
				Operator: models.OpEquals,
				Value:    signature,
			},
		},
		StatusCode: statusCode,
		Headers:    headers,
		Body:       body,
		Delay:      0,
		Enabled:    true,
		Recorded:   true,
		Origin:     origin,
	}

	_ = r.store.CreateResponseConfig(cfg)
}

// generateRecordedID creates a unique ID for a recorded ResponseConfig.
func generateRecordedID() string {
	return time.Now().Format("20060102150405") + "-" + uuid.New().String()[:8]
}
