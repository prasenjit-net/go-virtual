# Sequence Diagram: Incoming Mock Request

This diagram traces the full lifecycle of a request handled in **Mock Mode** —
the most common path through Go-Virtual.

```mermaid
sequenceDiagram
    participant C as API Client
    participant G as Gin Router
    participant PE as Proxy Engine
    participant SM as Session Manager
    participant CE as Condition Evaluator
    participant SE as Starlark Engine
    participant TE as Template Engine
    participant TS as Tracing Service
    participant WS as WebSocket Client

    C->>G: HTTP Request (+ optional X-Virtual-Session-Id)
    G->>PE: ServeHTTP(w, r)
    PE->>SM: ResolveSession(header)
    SM-->>PE: session (new UUID echoed if header missing)

    loop For each registered route (priority order)
        PE->>CE: Evaluate(conditions, request)
        CE-->>PE: matched / not matched
    end

    PE->>SE: RunBindings(script bindings, req, session)
    SE-->>PE: outputMap (keyed by OutputKey)

    PE->>TE: Render(responseTemplate, ctx{path,query,header,body,script,random,timestamp})
    TE-->>PE: rendered body + headers

    PE->>TS: RecordTrace(request, response, script logs)
    TS-->>WS: broadcast(TraceEvent via WebSocket)

    PE-->>G: WriteResponse(status, headers, body)
    G-->>C: HTTP Response (+ X-Virtual-Session-Id header)
```

## Notes

- **Session resolution**: if the `X-Virtual-Session-Id` header is absent or unknown, a new session
  is created and its UUID is echoed back in the response header. Sessions expire after the
  configured `inactivityTimeout` (default 30 min).
- **Condition evaluation**: response configs are evaluated in ascending priority order; the first
  match wins. If no config matches and `exampleFallback` is enabled, the OpenAPI example is used.
- **Script bindings**: executed in declaration order; each binding's return value is stored under
  `outputKey` and available in templates as `{{.script.<outputKey>.<field>}}`.
- **Tracing**: only emitted when tracing is enabled on the spec. Live trace events are broadcast
  to all connected WebSocket clients immediately after the response is written.
