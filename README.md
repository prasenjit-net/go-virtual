<p align="center">
  <img src="assets/logo-banner.svg" alt="go-virtual logo" width="520" />
</p>

<p align="center">
  <strong>API Mock &amp; Virtualization for OpenAPI 3</strong><br/>
  Configure dynamic mock responses, trace live traffic, and run chaos experiments — all from a single binary.
</p>

<p align="center">
  <a href="#quick-start">Quick Start</a> ·
  <a href="#features">Features</a> ·
  <a href="#configuration">Configuration</a> ·
  <a href="#api-reference">API Reference</a>
</p>

---

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
| `{{faker.name.first}}` | Faker first name | `{{faker.name.first}}` |
| `{{faker.name.last}}` | Faker last name | `{{faker.name.last}}` |
| `{{faker.name}}` | Faker full name | `{{faker.name}}` |
| `{{faker.email}}` | Faker email | `{{faker.email}}` |
| `{{faker.phone}}` | Faker phone | `{{faker.phone}}` |
| `{{faker.company.name}}` | Faker company name | `{{faker.company.name}}` |
| `{{faker.address.street}}` | Faker street address | `{{faker.address.street}}` |
| `{{faker.address.city}}` | Faker city | `{{faker.address.city}}` |
| `{{faker.address.state}}` | Faker state | `{{faker.address.state}}` |
| `{{faker.address.zip}}` | Faker postal code | `{{faker.address.zip}}` |
| `{{faker.internet.username}}` | Faker username | `{{faker.internet.username}}` |
| `{{faker.internet.domain}}` | Faker domain | `{{faker.internet.domain}}` |
| `{{faker.internet.url}}` | Faker URL | `{{faker.internet.url}}` |
| `{{faker.lorem.word}}` | Faker word | `{{faker.lorem.word}}` |
| `{{faker.lorem.sentence}}` | Faker sentence | `{{faker.lorem.sentence}}` |
| `{{faker.lorem.paragraph}}` | Faker paragraph | `{{faker.lorem.paragraph}}` |
| `{{timestamp}}` | Current Unix timestamp | - |
| `{{timestamp.iso}}` | Current ISO timestamp | - |

Faker outputs are deterministically seeded per request using the request path and query parameters.

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
