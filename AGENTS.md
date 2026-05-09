# AGENTS.md

Instructions for AI agents working in this repository.

## Project Overview

Arbiter is a self-hosted, OpenAI-compatible LLM proxy written in Go. It routes requests across local and cloud models based on availability and (later) tier priority. Single Docker container, YAML-configured, zero external dependencies.

Read `README.md` for full user-facing context before making changes.

## Repository Structure

```
arbiter/
├── cmd/
│   └── arbiter/
│       └── main.go          # entrypoint, config loading, server startup
├── internal/
│   ├── adapter/             # adapter interface + implementations
│   │   ├── adapter.go       # Adapter interface
│   │   ├── anthropic.go     # AnthropicAdapter
│   │   └── ollama.go        # OllamaAdapter
│   ├── router/              # tier routing logic + fallback chain
│   │   └── router.go
│   ├── cost/                # in-memory cost tracking
│   │   └── tracker.go
│   ├── health/              # /health handler
│   │   └── health.go
│   └── config/              # YAML config loading + validation
│       └── config.go
├── config.example.yaml      # annotated reference config
├── .env.example             # env var reference
├── compose.yml              # Docker Compose for end users
├── Dockerfile
├── go.mod
├── go.sum
└── README.md
```

## Tech Stack

- **Go** — idiomatic Go, standard project layout (`cmd/`, `internal/`)
- **Gin** — HTTP framework for routing and middleware
- **zap** — structured logging throughout (never use `fmt.Println` for logs)
- **gopkg.in/yaml.v3** — config parsing
- **go-playground/validator** — config struct validation
- No ORM, no DB in v0.1

## Core Interfaces

### Adapter

Every LLM provider implements this interface:

```go
type Adapter interface {
    Name() string // configured instance, e.g. "ollama-mistral" → X-Arbiter-Model
    Type() string // provider type, e.g. "ollama"           → X-Arbiter-Adapter
    Chat(ctx context.Context, req *ChatRequest) (*ChatResponse, error)
    ChatStream(ctx context.Context, req *ChatRequest) (<-chan ChatChunk, error)
    Health(ctx context.Context) error
    EstimateCost(req *ChatRequest, resp *ChatResponse) float64
}
```

When adding a new adapter, implement all methods. Never add provider-specific logic outside `internal/adapter/`.

### ChatRequest / ChatResponse

Must be OpenAI-compatible as of right now for v0.1. The router translates between Arbiter's internal types and each provider's wire format inside the adapter, never in the router or handlers.

## Routing Logic

Tiers are defined in `config.yaml` as ordered lists of adapter names. The router:

1. Looks up the tier by the `model` field in the incoming request
2. Tries adapters in order
3. If `fallback: true` and an adapter is unavailable (health check failed or returns error), moves to the next
4. Sets `X-Arbiter-Adapter`, `X-Arbiter-Model`, `X-Arbiter-Tier`, and `X-Arbiter-Fallback` response headers before returning

## Response Headers

Set these on every response, this is non-negotiable:

```go
c.Header("X-Arbiter-Adapter", result.AdapterType) // provider type, e.g. "ollama"
c.Header("X-Arbiter-Model", result.AdapterName)   // configured instance, e.g. "ollama-mistral"
c.Header("X-Arbiter-Tier", result.TierName)
if result.FallbackUsed {
    c.Header("X-Arbiter-Fallback", "true")
}
```

`X-Arbiter-Fallback` is only present when fallback was triggered. Headers must be set before streaming begins on SSE responses.

## Config Loading

Config is loaded once at startup from `/app/config.yaml` (the Docker mount path). Env vars referenced as `${VAR_NAME}` in the YAML must be expanded at load time using `os.ExpandEnv`. Fail fast with a clear error if a required field is missing or an env var is unset.

## Error Handling

- Return structured JSON errors on all HTTP error responses: `{"error": "message"}`
- Log errors with `zap` at appropriate levels.
  - `Error` for adapter failures, `Warn` for fallbacks, `Info` for request routing
- For errors in request handlers, recover and return 500
- Adapter errors should include the adapter name in the log context

## Docker

- The `Dockerfile` should produce a minimal image — use multi-stage build, final stage from `gcr.io/distroless/static`
- Default port is **9099**
- `HEALTHCHECK` in `Dockerfile` must call `GET /health`
- Config is always mounted as a volume — never bake config into the image
- `compose.yml` is for end users; do not modify it for dev convenience without a comment explaining the change

## Testing

- Unit tests live next to the code they test (`*_test.go`)
- Adapter tests should mock the HTTP layer — don't make real API calls in tests
- Test the router logic with mock adapters covering: happy path, single fallback, all adapters down
- Run tests: `go test ./...`

## What Not To Do

- Don't change the default port from 9099
- Don't import provider SDKs outside `internal/adapter/` — the core must stay provider-agnostic
- Don't log API keys, even partially

## Adding a New Adapter

1. Create `internal/adapter/{provider}.go`
2. Implement the `Adapter` interface
3. Register the adapter type string in `internal/config/config.go`
4. Add an entry to the adapter table in `README.md`
5. Add a commented example to `config.example.yaml`
6. Write unit tests with mocked HTTP
