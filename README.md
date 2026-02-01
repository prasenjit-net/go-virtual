# Go-Virtual

A powerful API proxy service for OpenAPI 3 specifications with configurable mock responses.

## Features

- **OpenAPI 3 Support**: Upload and manage multiple OpenAPI 3 specifications
- **Configurable Responses**: Design mock responses with conditions and priorities
- **Template Engine**: Dynamic response bodies and headers with variable substitution
- **Real-time Tracing**: Live request/response monitoring with WebSocket streaming
- **Statistics Dashboard**: Performance metrics and error tracking
- **Modern UI**: React-based admin interface with syntax highlighting

## Quick Start

### Prerequisites

- Go 1.21 or later
- Node.js 18 or later
- npm or yarn

### Installation

1. Clone the repository:
```bash
git clone https://github.com/prasenjit/go-virtual.git
cd go-virtual
```

2. Install dependencies:
```bash
make install-deps
```

3. Build the project:
```bash
make build
```

4. Run the server:
```bash
make run
```

5. Open the admin UI at `http://localhost:8080/_ui/`

## Development

### Running in Development Mode

Start the Go server in dev mode:
```bash
make dev-server
```

In a separate terminal, start the Vite dev server:
```bash
make dev-ui
```

The Go server runs on port 8080 and the Vite dev server on port 5173 with proxy to the Go server.

### Available Commands

```bash
make build        # Build everything (UI + Go binary)
make dev          # Run Go server in dev mode
make dev-ui       # Run Vite dev server
make test         # Run tests
make clean        # Clean build artifacts
make help         # Show all commands
```

## Configuration

Create a `config.yaml` file:

```yaml
server:
  port: 8080
  host: "0.0.0.0"

storage:
  type: "file"       # "memory" or "file"
  path: "./data"

tracing:
  maxTraces: 1000
  retention: "24h"

logging:
  level: "info"
  format: "json"
```

## API Reference

### Admin API

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/_api/specs` | List all specifications |
| POST | `/_api/specs` | Upload new specification |
| GET | `/_api/specs/:id` | Get specification details |
| PUT | `/_api/specs/:id` | Update specification |
| DELETE | `/_api/specs/:id` | Delete specification |
| PUT | `/_api/specs/:id/enable` | Enable specification |
| PUT | `/_api/specs/:id/disable` | Disable specification |
| PUT | `/_api/specs/:id/tracing` | Toggle tracing |
| GET | `/_api/specs/:id/operations` | List operations |
| GET | `/_api/operations/:id` | Get operation details |
| GET | `/_api/operations/:id/responses` | List response configs |
| POST | `/_api/operations/:id/responses` | Create response config |
| PUT | `/_api/responses/:id` | Update response config |
| DELETE | `/_api/responses/:id` | Delete response config |
| GET | `/_api/stats` | Get global statistics |
| GET | `/_api/traces` | List traces |
| WS | `/_api/traces/stream` | WebSocket for live traces |

## Template Variables

Use these variables in response bodies and headers:

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

## Condition Operators

| Operator | Description |
|----------|-------------|
| `eq` | Equals |
| `ne` | Not equals |
| `contains` | Contains substring |
| `notContains` | Does not contain |
| `regex` | Matches regex |
| `exists` | Value exists |
| `notExists` | Value does not exist |
| `gt` | Greater than |
| `lt` | Less than |
| `gte` | Greater than or equal |
| `lte` | Less than or equal |
| `startsWith` | Starts with |
| `endsWith` | Ends with |

## License

MIT License
