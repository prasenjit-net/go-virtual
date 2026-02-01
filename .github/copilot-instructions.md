# Go-Virtual Project Copilot Instructions

## Project Overview

Go-Virtual is an API proxy/mock service that virtualizes OpenAPI 3 specifications. It allows configuring custom responses based on request conditions, with support for templating, tracing, and statistics.

## Tech Stack

### Backend (Go 1.21+)
- **HTTP Framework**: Gin (`github.com/gin-gonic/gin`)
- **OpenAPI Parser**: kin-openapi (`github.com/getkin/kin-openapi/openapi3`)
- **WebSocket**: Gorilla WebSocket (`github.com/gorilla/websocket`)
- **JSON Path**: gjson (`github.com/tidwall/gjson`)
- **UUID**: Google UUID (`github.com/google/uuid`)
- **YAML**: gopkg.in/yaml.v3

### Frontend (React 18 + TypeScript)
- **Build Tool**: Vite
- **Styling**: TailwindCSS
- **State Management**: Zustand
- **Data Fetching**: @tanstack/react-query
- **Charts**: Recharts
- **Code Editor**: Monaco Editor (@monaco-editor/react)
- **Icons**: Lucide React
- **Drag & Drop**: @dnd-kit

## Project Structure

```
go-virtual/
├── cmd/server/          # Application entry point
├── internal/
│   ├── api/             # HTTP handlers and routing
│   ├── condition/       # Request condition evaluation
│   ├── config/          # Configuration loading
│   ├── models/          # Data models
│   ├── parser/          # OpenAPI 3 spec parser
│   ├── proxy/           # Dynamic proxy engine
│   ├── stats/           # Statistics collector
│   ├── storage/         # Data persistence (memory/file)
│   ├── template/        # Response templating engine
│   └── tracing/         # Request/response tracing
├── ui/                  # React frontend
│   └── src/
│       ├── components/  # React components
│       ├── services/    # API client
│       └── types/       # TypeScript interfaces
├── test/                # Test specs and data
├── ui.go                # Embedded UI filesystem
├── Makefile             # Build automation
└── config.yaml          # Default configuration
```

## Coding Conventions

### Go
- Use standard Go project layout with `cmd/` and `internal/`
- Keep packages focused and minimal
- Use interfaces for dependencies (storage, services)
- Error handling: return errors, don't panic
- Use `context.Context` for cancellation where appropriate
- Mutex naming: `mu` for single mutex, descriptive names for multiple
- Comments: GoDoc style for exported functions

### TypeScript/React
- Functional components with hooks
- TypeScript strict mode enabled
- Use React Query for server state
- Use Zustand for client state if needed
- TailwindCSS for styling (no CSS modules)
- Lucide icons (not other icon libraries)

## Key Patterns

### Template Variables
Response bodies and headers support these template variables:
- `{{.path.<param>}}` - Path parameters
- `{{.query.<param>}}` - Query parameters
- `{{.header.<name>}}` - Request headers
- `{{.body}}` - Full request body
- `{{.body.<jsonpath>}}` - JSON path extraction
- `{{.random.uuid}}`, `{{.random.int}}`, `{{.random.string}}`
- `{{.timestamp}}`, `{{.timestamp.unix}}`

### Condition Operators
- `eq`, `ne` - Equality
- `contains`, `not_contains` - String contains
- `regex` - Regular expression match
- `exists`, `not_exists` - Field existence
- `gt`, `gte`, `lt`, `lte` - Numeric comparison
- `in`, `not_in` - Value in list

### API Endpoints
- Admin API: `/_api/*` (specs, operations, responses, stats, traces)
- Admin UI: `/_ui/*` (embedded React SPA)
- Proxy: All other paths (matched against registered specs)

## Build Commands

```bash
make build          # Full production build (UI + Go binary)
make build-ui       # Build UI only
make build-go       # Build Go binary only
make dev-server     # Run Go server in dev mode
make dev-ui         # Run Vite dev server
make install-deps   # Install all dependencies
make clean          # Clean build artifacts
```

## Important Notes

1. **UI Embedding**: The UI is embedded in the Go binary via `//go:embed` directive in `ui.go`. Run `make build-ui` before `make build-go`.

2. **Dev Mode**: Use `-dev` flag to serve UI from filesystem instead of embedded files.

3. **Route Reloading**: Call `proxyEngine.ReloadRoutes()` after any spec/operation changes.

4. **Response Priority**: Lower priority number = higher precedence. Conditions are evaluated in priority order.

5. **Tracing**: Enable per-spec via the API. Traces are streamed via WebSocket at `/_api/traces/stream`.

## Testing

Test the proxy with the sample petstore spec:
```bash
# Upload spec
curl -X POST http://localhost:8080/_api/specs \
  -H "Content-Type: application/json" \
  -d '{"content": "$(cat test/petstore.yaml)", "name": "Pet Store"}'

# Test endpoint
curl http://localhost:8080/pets
```
