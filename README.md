# Meerkat

AI-powered observability agent that watches your infrastructure like a meerkat on sentinel duty.

## Architecture

```
Request (manual / webhook / scheduled)
    → Meerkat Service (creates pending report, spawns goroutine)
        → Analyzer (agentic AI loop with tool calls)
            → Datasources (Prometheus, Victoria Metrics, Loki, etc.)
        → Reporter (Slack, webhook, etc.)
    → Report (completed/failed)
```

## Quick Start

```bash
# Copy and edit config
cp config.example.yaml config.yaml

# Run migrations
go run ./cmd/meerkat-server migrate apply

# Start server
go run ./cmd/meerkat-server serve
```

## Configuration

```yaml
app:
  name: meerkat
  env: development

http:
  host: 0.0.0.0
  port: 8080

store:
  driver: postgres
  path: postgresql://localhost:5432/meerkat?sslmode=disable

datasources:
  - name: vm
    type: prometheus
    url: http://localhost:8428

analyzer:
  provider: openai
  url: https://api.openai.com
  api_key: ${OPENAI_API_KEY}
  model: gpt-4o
  max_iterations: 10
```

## CLI

```bash
# Trigger manual inspection
meerkat inspect -q "Check for error spikes in the last hour"

# List reports
meerkat report list

# Get specific report
meerkat report get <id>

# List datasources
meerkat datasource list
```

## Development

```bash
make gen        # Generate all code (ent, ogen, mocks)
make build      # Build binaries
make test       # Run unit tests
make test-e2e   # Run e2e tests (requires Docker)
make lint       # Run linter
```

## Docker

```bash
docker build -f build/docker/server.Dockerfile -t meerkat-server .
```

## License

Proprietary
