# Remaining improvements (comparison vs current implementation)

Date: 2026-02-09

This document compares prior improvement suggestions against the current implementation and lists what remains.

## Implemented since suggestions

- Response delay support (implemented in proxy response handling).
- Faker-style template variables with deterministic request seeding.
- Template variable documentation in the response editor UI.
- Dark mode toggle (light/dark/system) with persistent preference.
- Dark mode styling across core UI pages.

## Remaining items (not implemented yet)

### New features
1. Response scenarios / profiles (group response configs into named scenarios, switchable via UI/API).
2. Request recording & replay (capture live traffic and generate mocks).
3. GraphQL support (schema parsing + query-based responses).
4. Webhooks / callbacks simulation (async callback endpoints + triggers).
5. Request validation against OpenAPI (strict/permissive modes, detailed errors).
6. Chaos testing options (fault injection, latency jitter, timeouts, error rates).
7. Faker data expansion (more generators, locale support, custom seed controls).
8. Import/export bundles (specs + responses + settings).
9. Admin auth / RBAC / API keys.
10. Advanced templating (conditionals, loops, custom functions).
11. CSV/JSON export for stats and traces.
12. Version endpoint and health checks.
13. Rate limiting and quotas per spec/operation.
14. Multi-tenancy / environment isolation.

### Architecture & performance
1. Caching of parsed OpenAPI specs and compiled route patterns.
2. Hot reload for spec/config changes (file watcher + UI refresh).
3. Database storage option (Postgres/MySQL) for large datasets.
4. OpenTelemetry or Prometheus metrics export.
5. Structured logging with correlation IDs.

### Developer experience
1. CLI commands for spec upload, export/import, and config validation.
2. SDK/client generation from loaded specs.
3. Better docs: interactive tutorials, example templates, quick-start flows.
4. Docker/Helm/Kubernetes deployment assets.

### UI/UX enhancements
1. Global search across specs/operations/responses.
2. Bulk enable/disable and batch actions.
3. Visual condition builder (no JSON editing).
4. Response preview before saving.
5. Trace timeline view and comparison view.
6. Syntax highlighting/autocomplete for templates.

### Quality & reliability
1. Increased integration test coverage (parser, storage, API handlers).
2. Load-testing scenarios and benchmarks.
3. Fuzz tests for parser and template engine.
4. Better error messages for invalid configs and condition mismatches.

## Suggested next targets (highest impact)

1. Request validation (OpenAPI) with clear error reporting.
2. Import/export bundles for easy environment portability.
3. Response scenarios/profiles for rapid switching.
4. Prometheus metrics endpoint.
5. Visual condition builder in UI.
