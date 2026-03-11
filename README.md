# llama-swap

A lightweight API gateway for hot-swapping between multiple local LLM servers.

## Features

- **Unified API**: Single endpoint for multiple model servers
- **Hot Swapping**: Switch models without restarting
- **OpenAI Compatible**: Works with any OpenAI-compatible server
- **Zero Dependencies**: Pure Go with minimal external deps

## Quick Start

```bash
# Build
go build -o llama-swap ./cmd/main.go

# Run
./llama-swap -config config.yaml
```

## Configuration

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

## API Usage

### Chat Completions

```bash
# Using X-Model header
curl -X POST http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "X-Model: llama3" \
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
```

### List Models

```bash
curl http://localhost:8080/v1/models
```

### Health Check

```bash
curl http://localhost:8080/health
```

## Endpoints

| Method | Path | Description |
|--------|------|-------------|
| GET | /health | Health check |
| GET | /metrics | Request metrics |
| GET | /v1/models | List available models |
| POST | /v1/chat/completions | Chat completion |
| POST | /v1/completions | Text completion |

## Architecture

```
Client → llama-swap (:8080) → Local LLM Server
                              (llama.cpp/vllm/ollama)
```
