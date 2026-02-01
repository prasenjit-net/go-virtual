# Go-Virtual: OpenAPI 3 Proxy Service

## Project Overview

Go-Virtual is an API proxy service that virtualizes OpenAPI 3 APIs, allowing developers to mock, test, and simulate API responses based on configurable rules and conditions.

## Architecture Diagram

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                              Go-Virtual Server                               │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                              │
│  ┌─────────────────┐    ┌──────────────────┐    ┌──────────────────────┐   │
│  │   Admin UI      │    │   Admin API      │    │   Proxy Engine       │   │
│  │   (React/Vite)  │◄──►│   /_api/*        │    │   (Dynamic Routes)   │   │
│  │   /_ui/*        │    │                  │    │                      │   │
│  └─────────────────┘    └────────┬─────────┘    └──────────┬───────────┘   │
│                                  │                          │               │
│                                  ▼                          ▼               │
│  ┌─────────────────────────────────────────────────────────────────────────┤
│  │                        Core Services                                     │
│  │  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐    │
│  │  │ Spec        │  │ Response    │  │ Template    │  │ Statistics  │    │
│  │  │ Manager     │  │ Matcher     │  │ Engine      │  │ Collector   │    │
│  │  └─────────────┘  └─────────────┘  └─────────────┘  └─────────────┘    │
│  │  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐                     │
│  │  │ Tracing     │  │ Condition   │  │ OpenAPI     │                     │
│  │  │ Service     │  │ Evaluator   │  │ Parser      │                     │
│  │  └─────────────┘  └─────────────┘  └─────────────┘                     │
│  └─────────────────────────────────────────────────────────────────────────┤
│                                  │                                          │
│                                  ▼                                          │
│  ┌─────────────────────────────────────────────────────────────────────────┤
│  │                        Storage Layer                                     │
│  │  ┌─────────────────────────┐    ┌─────────────────────────┐            │
│  │  │   In-Memory Store       │    │   File-based Store      │            │
│  │  │   (Runtime Cache)       │    │   (Persistence)         │            │
│  │  └─────────────────────────┘    └─────────────────────────┘            │
│  └─────────────────────────────────────────────────────────────────────────┤
│                                                                              │
└─────────────────────────────────────────────────────────────────────────────┘
```

## Core Components

### 1. Proxy Engine
- Dynamically routes incoming requests to matched OpenAPI operations
- Evaluates conditions against request data
- Returns templated responses based on priority

### 2. Spec Manager
- Parses and validates OpenAPI 3 specifications
- Manages spec lifecycle (enable/disable)
- Maintains operation registry

### 3. Response Matcher
- Matches incoming requests to configured responses
- Evaluates conditions in priority order
- Supports fallback to default responses

### 4. Template Engine
- Supports variable substitution in response bodies and headers
- Variables from: path params, query params, headers, body, random generators

### 5. Condition Evaluator
- Evaluates request against configured conditions
- Supports: equals, contains, regex, exists, comparison operators
- Supports logical operators: AND, OR, NOT

### 6. Tracing Service
- Captures full request/response data when enabled
- Streams traces via WebSocket to Admin UI
- Stores trace history for review

### 7. Statistics Collector
- Tracks request counts per spec/operation
- Measures response times
- Records error rates

## Data Models

### Spec
```go
type Spec struct {
    ID          string            `json:"id"`
    Name        string            `json:"name"`
    Version     string            `json:"version"`
    Description string            `json:"description"`
    Content     string            `json:"content"`      // Raw OpenAPI spec
    BasePath    string            `json:"basePath"`     // Mounted path prefix
    Enabled     bool              `json:"enabled"`
    Tracing     bool              `json:"tracing"`      // Enable request tracing
    CreatedAt   time.Time         `json:"createdAt"`
    UpdatedAt   time.Time         `json:"updatedAt"`
    Operations  []Operation       `json:"operations"`
}
```

### Operation
```go
type Operation struct {
    ID          string            `json:"id"`
    SpecID      string            `json:"specId"`
    Method      string            `json:"method"`       // GET, POST, etc.
    Path        string            `json:"path"`         // /users/{id}
    OperationID string            `json:"operationId"`
    Summary     string            `json:"summary"`
    Responses   []ResponseConfig  `json:"responses"`
}
```

### ResponseConfig
```go
type ResponseConfig struct {
    ID          string            `json:"id"`
    OperationID string            `json:"operationId"`
    Name        string            `json:"name"`
    Priority    int               `json:"priority"`     // Lower = higher priority
    Conditions  []Condition       `json:"conditions"`   // All must match (AND)
    StatusCode  int               `json:"statusCode"`
    Headers     map[string]string `json:"headers"`      // Templated
    Body        string            `json:"body"`         // Templated
    Delay       int               `json:"delay"`        // Response delay in ms
    Enabled     bool              `json:"enabled"`
}
```

### Condition
```go
type Condition struct {
    Source      string            `json:"source"`       // path, query, header, body
    Key         string            `json:"key"`          // Parameter name or JSONPath
    Operator    string            `json:"operator"`     // eq, ne, contains, regex, exists, gt, lt
    Value       string            `json:"value"`        // Expected value
}
```

### Trace
```go
type Trace struct {
    ID            string            `json:"id"`
    SpecID        string            `json:"specId"`
    OperationID   string            `json:"operationId"`
    Timestamp     time.Time         `json:"timestamp"`
    Duration      int64             `json:"duration"`     // nanoseconds
    Request       TraceRequest      `json:"request"`
    Response      TraceResponse     `json:"response"`
    MatchedConfig string            `json:"matchedConfig"`
}

type TraceRequest struct {
    Method      string              `json:"method"`
    URL         string              `json:"url"`
    Headers     map[string][]string `json:"headers"`
    Body        string              `json:"body"`
}

type TraceResponse struct {
    StatusCode  int                 `json:"statusCode"`
    Headers     map[string][]string `json:"headers"`
    Body        string              `json:"body"`
}
```

## Template Variables

Templates use `{{variable}}` syntax with the following available variables:

| Variable | Description | Example |
|----------|-------------|---------|
| `{{path.paramName}}` | URL path parameter | `{{path.userId}}` |
| `{{query.paramName}}` | Query string parameter | `{{query.page}}` |
| `{{header.headerName}}` | Request header | `{{header.Authorization}}` |
| `{{body.jsonPath}}` | JSONPath into request body | `{{body.user.name}}` |
| `{{random.uuid}}` | Random UUID | - |
| `{{random.int(min,max)}}` | Random integer | `{{random.int(1,100)}}` |
| `{{random.string(len)}}` | Random string | `{{random.string(10)}}` |
| `{{timestamp}}` | Current Unix timestamp | - |
| `{{timestamp.iso}}` | Current ISO timestamp | - |
| `{{timestamp.format(layout)}}` | Formatted timestamp | `{{timestamp.format(2006-01-02)}}` |

## API Endpoints

### Admin API (`/_api/`)

#### Specs
- `GET /_api/specs` - List all specs
- `POST /_api/specs` - Upload new spec
- `GET /_api/specs/{id}` - Get spec details
- `PUT /_api/specs/{id}` - Update spec
- `DELETE /_api/specs/{id}` - Delete spec
- `PUT /_api/specs/{id}/enable` - Enable spec
- `PUT /_api/specs/{id}/disable` - Disable spec
- `PUT /_api/specs/{id}/tracing` - Toggle tracing

#### Operations
- `GET /_api/specs/{id}/operations` - List operations for spec
- `GET /_api/operations/{id}` - Get operation details

#### Response Configs
- `GET /_api/operations/{id}/responses` - List response configs
- `POST /_api/operations/{id}/responses` - Create response config
- `PUT /_api/responses/{id}` - Update response config
- `DELETE /_api/responses/{id}` - Delete response config
- `PUT /_api/responses/{id}/priority` - Update priority

#### Statistics
- `GET /_api/stats` - Global statistics
- `GET /_api/stats/specs/{id}` - Spec statistics
- `GET /_api/stats/operations/{id}` - Operation statistics

#### Tracing
- `GET /_api/traces` - List recent traces
- `GET /_api/traces/{id}` - Get trace details
- `DELETE /_api/traces` - Clear traces
- `WS /_api/traces/stream` - WebSocket for live traces

## Project Structure

```
go-virtual/
├── cmd/
│   └── server/
│       └── main.go              # Application entry point
├── internal/
│   ├── api/
│   │   ├── handler.go           # Admin API handlers
│   │   ├── middleware.go        # API middleware
│   │   └── router.go            # API router setup
│   ├── config/
│   │   └── config.go            # Application configuration
│   ├── models/
│   │   ├── spec.go              # Spec model
│   │   ├── operation.go         # Operation model
│   │   ├── response.go          # Response config model
│   │   ├── condition.go         # Condition model
│   │   ├── trace.go             # Trace model
│   │   └── stats.go             # Statistics model
│   ├── parser/
│   │   └── openapi.go           # OpenAPI 3 parser
│   ├── proxy/
│   │   ├── engine.go            # Proxy engine
│   │   ├── matcher.go           # Request/response matcher
│   │   └── router.go            # Dynamic router
│   ├── storage/
│   │   ├── interface.go         # Storage interface
│   │   ├── memory.go            # In-memory storage
│   │   └── file.go              # File-based storage
│   ├── template/
│   │   └── engine.go            # Template processing
│   ├── condition/
│   │   └── evaluator.go         # Condition evaluation
│   ├── tracing/
│   │   ├── service.go           # Tracing service
│   │   └── websocket.go         # WebSocket handler
│   └── stats/
│       └── collector.go         # Statistics collection
├── ui/                          # React/Vite project
│   ├── src/
│   │   ├── components/
│   │   │   ├── Layout/
│   │   │   ├── Dashboard/
│   │   │   ├── SpecManager/
│   │   │   ├── OperationList/
│   │   │   ├── ResponseDesigner/
│   │   │   ├── ConditionBuilder/
│   │   │   ├── TraceViewer/
│   │   │   └── common/
│   │   ├── hooks/
│   │   ├── services/
│   │   ├── store/
│   │   ├── types/
│   │   ├── App.tsx
│   │   └── main.tsx
│   ├── index.html
│   ├── package.json
│   ├── tsconfig.json
│   └── vite.config.ts
├── ui/dist/                     # Built UI (generated)
├── embedded/
│   └── ui.go                    # Embedded UI files
├── data/                        # Persistent storage
├── Makefile
├── go.mod
├── go.sum
└── README.md
```

## Build Process

### Development Mode
```bash
# Terminal 1: Start Go server with hot reload
make dev-server

# Terminal 2: Start Vite dev server
make dev-ui
```

In dev mode:
- Go server runs on :8080
- Vite dev server runs on :5173 with proxy to Go server
- UI requests to `/_ui/` are served from Vite

### Production Build
```bash
make build
```

This will:
1. Build React UI with Vite (`npm run build`)
2. Embed built UI into Go binary using `embed` package
3. Compile Go binary with embedded UI

## Configuration

```yaml
# config.yaml
server:
  port: 8080
  host: "0.0.0.0"

storage:
  type: "file"           # "memory" or "file"
  path: "./data"

tracing:
  maxTraces: 1000        # Max traces to keep in memory
  retention: "24h"       # Trace retention period

logging:
  level: "info"
  format: "json"
```

## UI Features

### Dashboard
- Total requests (today/week/month)
- Active specs count
- Operations count
- Response time charts
- Error rate trends
- Top operations by request count

### Spec Manager
- Upload OpenAPI 3 specs (YAML/JSON)
- List all specs with status
- Enable/disable specs
- View spec details and operations
- Delete specs
- Configure base path

### Response Designer
- Select operation
- Create multiple response configs
- Drag-and-drop priority ordering
- Condition builder UI
- JSON/text body editor with syntax highlighting
- Header editor
- Response preview
- Test response matching

### Trace Viewer
- Real-time trace stream (WebSocket)
- Filter by spec/operation
- Search traces
- View full request/response details
- Request/response body viewer with formatting
- Timeline view

## Dependencies

### Go
- `github.com/gin-gonic/gin` - HTTP router
- `github.com/getkin/kin-openapi` - OpenAPI 3 parser
- `github.com/gorilla/websocket` - WebSocket support
- `github.com/google/uuid` - UUID generation
- `gopkg.in/yaml.v3` - YAML parsing
- `github.com/tidwall/gjson` - JSON path queries

### React
- `react-router-dom` - Routing
- `@tanstack/react-query` - Data fetching
- `zustand` - State management
- `@monaco-editor/react` - Code editor
- `recharts` - Charts
- `tailwindcss` - Styling
- `lucide-react` - Icons
- `@dnd-kit/core` - Drag and drop

## Implementation Phases

### Phase 1: Core Foundation
1. Project setup and structure
2. OpenAPI 3 parser
3. Basic proxy engine
4. In-memory storage
5. Admin API skeleton

### Phase 2: Response System
1. Response configuration
2. Condition evaluator
3. Template engine
4. Priority-based matching

### Phase 3: Admin UI
1. UI project setup
2. Dashboard
3. Spec management
4. Response designer

### Phase 4: Advanced Features
1. Tracing system
2. WebSocket streaming
3. Statistics collection
4. File-based persistence

### Phase 5: Polish
1. Error handling
2. Validation
3. Documentation
4. Testing
