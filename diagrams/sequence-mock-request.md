# Sequence Diagram: Incoming Request Resolution

This diagram traces the current request lifecycle in Go-Virtual, including
saved-response lookup, recorded-response replay, spec-level fallback policy,
global AI scenarios, and proxy recording.

```mermaid
sequenceDiagram
    participant C as API Client
    participant G as Gin Router
    participant PE as Proxy Engine
    participant SM as Session Manager
    participant ST as Storage
    participant CE as Condition Evaluator
    participant SE as Starlark Engine
    participant TE as Template Engine
    participant AI as AI Generator
    participant UP as Upstream API
    participant TS as Tracing Service
    participant WS as WebSocket Client

    C->>G: HTTP Request (+ optional X-Virtual-Session-Id / X-Virtual-AI-Scenario)
    G->>PE: ServeHTTP(w, r)
    PE->>SM: ResolveSession(header)
    SM-->>PE: session (new UUID echoed if header missing)
    PE->>PE: Compute signature\n(ignore all X-Virtual-* headers)
    PE->>ST: Load operation + saved responses

    loop Configured/manual + AI-generated responses (priority order)
        PE->>CE: Evaluate(conditions, request)
        CE-->>PE: matched / not matched
    end

    alt Configured response matched
        PE->>SE: RunBindings(script bindings, req, session)
        SE-->>PE: outputMap (keyed by OutputKey)
        PE->>TE: Render(responseTemplate, ctx{path,query,header,body,script,random,timestamp})
        TE-->>PE: rendered body + headers
    else No configured response matched
        PE->>ST: Find recorded response by signature
        ST-->>PE: recorded match / miss

        alt Recorded response matched
            PE->>PE: Return recorded response
        else No recorded response matched
            PE->>CE: Evaluate spec mode policy per request\n(AI conditions, then proxy conditions)
            CE-->>PE: selected fallback mode

            alt AI fallback selected
                PE->>AI: GenerateRuntimeResponse(req, operation, scenario)
                AI-->>PE: generated status + headers + body
                PE->>ST: Save generated response (origin=ai)
            else Proxy fallback selected
                PE->>UP: Forward request to upstream
                UP-->>PE: upstream response
                PE->>ST: Save recorded response (origin=proxy)
            else Standard fallback selected
                PE->>PE: Use example/default success response
            end
        end
    end

    PE->>TS: RecordTrace(request, response,\nresponse tier, mode, scenario, script logs)
    TS-->>WS: broadcast(TraceEvent via WebSocket)

    PE-->>G: WriteResponse(status, headers, body)
    G-->>C: HTTP Response (+ X-Virtual-Session-Id header)
```

## Notes

- **Session resolution**: if the `X-Virtual-Session-Id` header is absent or unknown, a new session
  is created and its UUID is echoed back in the response header. Sessions expire after the
  configured `inactivityTimeout` (default 30 min).
- **Saved response order**: configured/manual and pre-generated AI responses are tried first by
  priority, then recorded responses are matched by signature, then runtime fallback policy runs.
- **Spec fallback policy**: AI and proxy are enabled or disabled at the spec level, with
  optional per-request conditions evaluated in sequence. Standard fallback is always available last.
- **AI scenarios**: callers can send `X-Virtual-AI-Scenario` to steer runtime AI output toward a
  named global scenario (for example `success`, `client_error`, `server_error`, or custom cases).
- **Script bindings**: executed in declaration order; each binding's return value is stored under
  `outputKey` and available in templates as `{{.script.<outputKey>.<field>}}`.
- **Recorded response hashing**: all `X-Virtual-*` headers are ignored when computing the request
  signature so internal control headers do not fragment replayed recordings.
- **Tracing**: only emitted when tracing is enabled on the spec. Traces now include response tier,
  selected fallback mode, AI skip reasons, proxy skip reasons, and requested/applied AI scenarios.
