# Arbiter

A self-hosted, LLM proxy to route requests across local and cloud models based on availability. Shipped in a Docker container.

```
curl http://localhost:9099/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{"model": "fast", "messages": [{"role": "user", "content": "Hello"}]}'
```

## Why Arbiter

I created Arbiter to be able to fold in my local hardware into my AI Agent stack without having to worry about ensuring the hardware is available. Model categories are completely customizable, but internally designed for a tier based routing system.

## Quickstart

**1. Clone and configure**

```bash
git clone https://github.com/mithunb9/arbiter
cd arbiter
cp config.example.yaml config.yaml
cp .env.example .env
```

**2. Run**

```bash
docker compose up -d
```

**3. Point your tooling at it**

Set your LLM client's base URL to `http://localhost:9099` and use the different "tier" names you customized as the model:

```
base_url: http://localhost:9099
model: fast
```

Designed to work with Cursor, Zed, Zenith, LangChain, or any OpenAI-compatible client.

## Configuration

Arbiter is configured via a single `config.yaml` file mounted into the container. See `config.example.yaml` for a fully annotated reference.

### Structure

```yaml
server:
  port: 9099

tiers:
  - name: fast
    adapters: [ollama-mistral, claude-haiku]
    fallback: true

  - name: cloud
    adapters: [claude-haiku, claude-sonnet]
    fallback: true

  - name: premium
    adapters: [claude-opus]

adapters:
  - name: ollama-mistral
    type: ollama
    base_url: http://host.docker.internal:11434
    model: mistral

  - name: claude-haiku
    type: anthropic
    api_key: ${ANTHROPIC_API_KEY}
    model: claude-haiku-4-5

  - name: claude-sonnet
    type: anthropic
    api_key: ${ANTHROPIC_API_KEY}
    model: claude-sonnet-4-6

  - name: claude-opus
    type: anthropic
    api_key: ${ANTHROPIC_API_KEY}
    model: claude-opus-4-7
```

### API keys

Pass keys via environment variables only. In `.env`:

```
ANTHROPIC_API_KEY=sk-ant-...
```

Docker Compose passes these through automatically.

## API

### `POST /v1/chat/completions`

OpenAI-compatible chat endpoint. Pass a tier name as the `model` field.

```json
{
  "model": "fast",
  "messages": [{ "role": "user", "content": "Explain recursion" }],
  "stream": true
}
```

Supports streaming (SSE) and non-streaming responses.

### `GET /health`

Shallow health check. Returns `200 OK` if the process is running.

```json
{ "status": "ok" }
```

## Response Headers

Every response from Arbiter includes routing metadata:

| Header               | Example          | Description                              |
| -------------------- | ---------------- | ---------------------------------------- |
| `X-Arbiter-Adapter`  | `ollama`         | Which adapter handled the request        |
| `X-Arbiter-Model`    | `ollama-mistral` | Which model handled the request          |
| `X-Arbiter-Tier`     | `fast`           | Which tier was requested                 |
| `X-Arbiter-Fallback` | `true`           | Only present when fallback was triggered |

These headers are available on both streaming and non-streaming responses.

## Docker

### Compose (recommended)

```yaml
# compose.yml
services:
  arbiter:
    image: ghcr.io/mithunb9/arbiter:latest
    ports:
      - "9099:9099"
    volumes:
      - ./config.yaml:/app/config.yaml:ro
    environment:
      - ANTHROPIC_API_KEY=${ANTHROPIC_API_KEY}
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:9099/health"]
      interval: 30s
      timeout: 5s
      retries: 3
    restart: unless-stopped
```

```bash
docker compose up -d
docker compose logs -f
```

### Run directly

```bash
docker run -d \
  -p 9099:9099 \
  -v ./config.yaml:/app/config.yaml:ro \
  -e ANTHROPIC_API_KEY=sk-ant-... \
  ghcr.io/mithunb9/arbiter:latest
```

## Adapters

| Type            | Description                                           | Status |
| --------------- | ----------------------------------------------------- | ------ |
| `ollama`        | Local Ollama instance                                 | v0.1   |
| `anthropic`     | Claude API via official SDK (all models)              | v0.1   |
| `openai_compat` | Groq, Together, LM Studio, any OpenAI-compat endpoint | v0.2   |

## Roadmap

- **v0.1** — OpenAI-compat proxy, Anthropic + Ollama adapters, tier routing, health endpoints, Docker (current)
- **v0.2** — OpenAI-compat adapter, SQLite cost persistence, health-check-based fallback
- **v0.3** — Budget cap enforcement, Prometheus `/metrics`, cost + latency response headers
- **v0.4** — Investigate complexity classifier (heuristic-first)

## License

MIT — see [LICENSE](LICENSE)
