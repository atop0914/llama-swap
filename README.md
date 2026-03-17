# llama-swap

A lightweight API gateway for hot-swapping between multiple local LLM servers via a unified OpenAI-compatible API.

## Features

- **Unified API**: Single endpoint for multiple model servers
- **Hot Swapping**: Switch models on-the-fly using `X-Model` header
- **OpenAI Compatible**: Works with any OpenAI-compatible server (llama.cpp, vllm, ollama, etc.)
- **Streaming Support**: Full support for Server-Sent Events (SSE) streaming
- **Minimal Dependencies**: Pure Go with minimal external deps

## Quick Start

```bash
# Build
go build -o llama-swap ./cmd/main.go

# Run with custom config
./llama-swap -config config.yaml

# Run with default config.yaml
./llama-swap
```

## Configuration

### config.yaml

```yaml
server:
  host: "0.0.0.0"
  port: 8080

upstreams:
  - name: "llama3"
    url: "http://localhost:8080"
    api_key: ""
  - name: "qwen"
    url: "http://localhost:8000"
    api_key: ""
  - name: "ollama"
    url: "http://localhost:11434"
    api_key: ""

default_upstream: "llama3"
```

### Configuration Options

| Field | Type | Description | Default |
|-------|------|-------------|---------|
| `server.host` | string | Listen host | `0.0.0.0` |
| `server.port` | int | Listen port | `8080` |
| `upstreams` | array | List of upstream servers | - |
| `upstreams[].name` | string | Model name (used in X-Model header) | - |
| `upstreams[].url` | string | Base URL of upstream server | - |
| `upstreams[].api_key` | string | Optional API key for auth | - |
| `default_upstream` | string | Default model when X-Model not specified | First in list |

## API Usage

### Chat Completions

```bash
# Using X-Model header to specify model
curl -X POST http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "X-Model: qwen" \
  -d '{
    "messages": [{"role": "user", "content": "Hello!"}],
    "stream": false
  }'

# Using default model
curl -X POST http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "messages": [{"role": "user", "content": "Hello!"}]
  }'

# Streaming response
curl -X POST http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "X-Model: llama3" \
  -H "Accept: text/event-stream" \
  -d '{
    "messages": [{"role": "user", "content": "Tell me a story"}],
    "stream": true
  }'
```

### Text Completions

```bash
curl -X POST http://localhost:8080/v1/completions \
  -H "Content-Type: application/json" \
  -H "X-Model: ollama" \
  -d '{
    "prompt": "Once upon a time",
    "max_tokens": 100
  }'
```

### List Models

```bash
curl http://localhost:8080/v1/models
```

Response:
```json
{
  "object": "list",
  "data": [
    {"id": "llama3", "object": "model", "owned_by": "local"},
    {"id": "qwen", "object": "model", "owned_by": "local"},
    {"id": "ollama", "object": "model", "owned_by": "local"}
  ]
}
```

### Health Check

```bash
curl http://localhost:8080/health
```

## API Reference

### Endpoints

| Method | Path | Description |
|--------|------|-------------|
| GET | `/health` | Health check (returns `{"status":"ok"}`) |
| GET | `/metrics` | Request metrics (P2) |
| GET | `/v1/models` | List available models |
| POST | `/v1/chat/completions` | Chat completion |
| POST | `/v1/completions` | Text completion |

### Headers

| Header | Required | Description |
|--------|----------|-------------|
| `X-Model` | No | Target model/upstream name |
| `Authorization` | No | Bearer token (passed to upstream) |
| `Content-Type` | Yes* | Must be `application/json` |
| `Accept` | No | Use `text/event-stream` for streaming |

*Required for POST requests

### Error Responses

Errors are returned in OpenAI-compatible format:

```json
{
  "error": {
    "message": "upstream error details",
    "type": "upstream_error",
    "code": "502"
  }
}
```

## Architecture

```
                    +------------------+
                    |   Client App    |
                    +--------+---------+
                             |
                             v
                    +------------------+
                    |  llama-swap      |
                    |  :8080           |
                    +--------+---------+
                             |
            +----------------+----------------+
            |                |                |
            v                v                v
     +------------+    +------------+    +------------+
     | llama.cpp  |    |   vllm     |    |  ollama    |
     | :8081      |    |   :8000    |    | :11434     |
     +------------+    +------------+    +------------+
```

## Project Structure

```
llama-swap/
├── cmd/
│   └── main.go           # Entry point
├── internal/
│   ├── config/
│   │   └── config.go     # Configuration loading
│   ├── proxy/
│   │   └── proxy.go      # Proxy logic
│   ├── handler/
│   │   └── handler.go    # HTTP handlers
│   ├── upstream/
│   │   └── upstream.go   # Upstream management
│   └── logger/
│       └── logger.go      # Structured logging
├── config.yaml           # Configuration file
├── README.md
└── go.mod
```

## Testing

```bash
# Run all tests
go test ./...

# Run tests with verbose output
go test ./... -v

# Run tests for specific package
go test ./internal/proxy/... -v
```

## License

MIT
