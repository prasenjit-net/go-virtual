# Plan: Admin API Authentication Middleware

**Priority:** P2  
**Effort:** M (1–2 days)  
**Target release:** v1.4.0  
**Related:** [improvement-roadmap.md § 3.1](improvement-roadmap.md)

---

## Problem

The entire `/_api/*` admin surface is completely unauthenticated. Anyone who can reach the server port can:

- Read all specs, scripts, and store entries.
- Modify, delete, or disable specs.
- Execute arbitrary Starlark scripts via `POST /_api/scripts/:id/test`.
- Restore archives that wipe existing data.

For local development this is fine, but for:
- Shared team environments
- Cloud / Kubernetes deployments
- Any publicly-accessible instance

…the lack of auth is a security gap.

---

## Goal

1. Add **optional** API-key authentication, **disabled by default** (zero behaviour change for existing deployments).
2. Support multiple named keys with optional scopes (for future RBAC).
3. Admin UI stores the key in `localStorage` and sends it automatically.
4. The Prometheus endpoint (`/_prometheus`) and proxy endpoints are **excluded** from auth.

---

## Config Schema

```yaml
# config.yaml
auth:
  enabled: false          # default off; set true to enforce auth
  apiKeys:
    - name: "admin"
      key: "sk_xxxxxxxxxxxxxxxxxxxx"
      scopes: []          # empty = full access (all scopes)
    - name: "readonly"
      key: "sk_yyyyyyyyyyyyyyyyyyyy"
      scopes: ["read"]
```

### New config types

```go
// AuthConfig controls admin API authentication.
type AuthConfig struct {
    Enabled bool        `yaml:"enabled"`
    APIKeys []APIKeyConfig `yaml:"apiKeys"`
}

// APIKeyConfig represents a single API key.
type APIKeyConfig struct {
    Name   string   `yaml:"name"`
    Key    string   `yaml:"key"`
    Scopes []string `yaml:"scopes"` // empty = all scopes
}
```

`Config.Auth AuthConfig` is added to the main `Config` struct.

---

## Middleware Implementation

### Package: `internal/api/middleware/`

New file: `internal/api/middleware/auth.go`

```go
package middleware

import (
    "net/http"
    "github.com/gin-gonic/gin"
    "github.com/prasenjit/go-virtual/internal/config"
)

// APIKeyAuth returns a Gin middleware that enforces API-key authentication.
// When cfg.Enabled is false, the middleware is a no-op pass-through.
func APIKeyAuth(cfg config.AuthConfig) gin.HandlerFunc {
    if !cfg.Enabled {
        return func(c *gin.Context) { c.Next() }
    }

    // Build index for O(1) lookup
    keys := make(map[string]config.APIKeyConfig, len(cfg.APIKeys))
    for _, k := range cfg.APIKeys {
        keys[k.Key] = k
    }

    return func(c *gin.Context) {
        token := extractToken(c)
        if token == "" {
            c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
                "error": "authentication required",
                "hint":  "provide key via 'Authorization: Bearer <key>' or 'X-Api-Key: <key>'",
            })
            return
        }

        apiKey, ok := keys[token]
        if !ok {
            c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
                "error": "invalid api key",
            })
            return
        }

        // Attach key metadata for downstream scope checks (future use)
        c.Set("apiKeyName", apiKey.Name)
        c.Set("apiKeyScopes", apiKey.Scopes)
        c.Next()
    }
}

// extractToken extracts the bearer token from the request.
// Checks Authorization header first, then X-Api-Key.
func extractToken(c *gin.Context) string {
    auth := c.GetHeader("Authorization")
    if strings.HasPrefix(auth, "Bearer ") {
        return strings.TrimPrefix(auth, "Bearer ")
    }
    return c.GetHeader("X-Api-Key")
}
```

---

## Router Changes

`router.go` → `setupRoutes()`:

```go
// Apply auth middleware to admin API routes only
authMW := middleware.APIKeyAuth(r.authConfig)
api := r.engine.Group("/_api", authMW)
```

`/_prometheus` and the proxy `NoRoute` handler are **outside** the `/_api` group and are not covered by the middleware.

`/_ui/*` static file serving is also excluded (the UI itself has no sensitive data; the API calls it makes will be auth-gated).

---

## Admin UI Changes

### Settings page / Login flow

When `GET /_api/health` returns `401`, the UI redirects to a simple **API Key** entry screen:

```
┌──────────────────────────────────────────────┐
│  go-virtual — Authentication Required        │
│                                              │
│  API Key:  [____________________________]    │
│                                              │
│            [Connect]                         │
└──────────────────────────────────────────────┘
```

On submit:
1. Store the key in `localStorage` under `gv_api_key`.
2. Retry `GET /_api/health` with the key — if 200, proceed.
3. On failure, show an error.

### API client (`services/api.ts`)

Add an interceptor that attaches the stored key to every request:

```ts
const apiKey = localStorage.getItem('gv_api_key') ?? ''

axios.interceptors.request.use(cfg => {
  if (apiKey) cfg.headers['X-Api-Key'] = apiKey
  return cfg
})
```

On 401 from any API call, clear the stored key and show the login screen.

### Branding endpoint exception

`GET /_api/branding` should be **excluded** from auth so the login screen can display the custom app title without a key. Implement this by adding the branding route before the auth middleware group, or by adding an explicit exemption.

---

## Security Notes

- **Key storage:** Keys are stored in plain-text in `config.yaml`. For production deployments, document the use of environment variable substitution (e.g. `key: "${GV_API_KEY}"`). Add env-var interpolation support to `config.Load()`.
- **HTTPS:** Recommend using TLS alongside auth — API keys over plain HTTP can be sniffed. Document this clearly.
- **Key rotation:** Operators can add a new key, update clients, then remove the old key without downtime (multiple keys supported).
- **No rate limiting on auth failures:** Consider adding a simple in-memory lockout after N failed attempts (configurable, default off).

---

## Test Plan

### Unit tests — `internal/api/middleware/auth_test.go`

```go
TestAPIKeyAuth_Disabled_AllowsAll        // enabled=false → 200 always
TestAPIKeyAuth_ValidBearerToken          // Authorization: Bearer sk_x → 200
TestAPIKeyAuth_ValidXApiKey              // X-Api-Key: sk_x → 200
TestAPIKeyAuth_InvalidKey                // wrong key → 403
TestAPIKeyAuth_MissingKey                // no header → 401
TestAPIKeyAuth_EmptyKeyList              // auth enabled, no keys → always 403
```

### Integration tests

- Start server with `auth.enabled: true` and one key.
- `GET /_api/specs` without key → 401.
- `GET /_api/specs` with correct key → 200.
- `GET /_api/branding` without key → 200 (exempted).
- `GET /_prometheus` without key → 200 (exempted).

---

## Acceptance Criteria

- [ ] `AuthConfig` added to `Config` struct
- [ ] `APIKeyAuth` middleware in `internal/api/middleware/auth.go`
- [ ] Middleware is a no-op when `auth.enabled: false`
- [ ] `/_api/*` routes require auth when enabled
- [ ] `/_prometheus`, proxy routes, and `/_api/branding` exempt from auth
- [ ] Admin UI shows login screen on 401
- [ ] Admin UI sends key header on all API requests
- [ ] Middleware tests pass
- [ ] Default config has `auth.enabled: false`
- [ ] README / docs updated with auth configuration section
