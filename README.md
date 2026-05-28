# Meerkat

AI-powered observability agent that watches your infrastructure like a meerkat on sentinel duty.

## Architecture

Meerkat consists of **three independently deployable services**:

```
┌─────────────────────────────────────────────────────────────┐
│                        Meerkat 시스템                        │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│  ┌──────────────┐     gRPC      ┌────────────────────────┐ │
│  │   Analyzer   │◄─────────────►│     MeerkatLogs        │ │
│  │   Server     │   Search/     │  ┌──────────────────┐  │ │
│  │  (HTTP API)  │   GetContext  │  │  gRPC Server     │  │ │
│  └──────────────┘               │  │  - search_logs   │  │ │
│       │                         │  │  - Ingest        │  │ │
│       │ AI Agent                │  │  - GetContext    │  │ │
│       ▼                         │  └──────────────────┘  │ │
│  ┌──────────────┐               │  ┌──────────────────┐  │ │
│  │  LLM + Tools │               │  │  OTLP Receiver   │  │ │
│  │              │               │  │  (Port :4317)    │  │ │
│  │ search_logs  │               │  └──────────────────┘  │ │
│  │  prometheus  │               │  ┌──────────────────┐  │ │
│  │  loki        │               │  │  Smart Pipeline  │  │ │
│  │  victorialogs│               │  │  - Filter (mode) │  │ │
│  │  ...         │               │  │  - Drain Extract │  │ │
│  └──────────────┘               │  │  - Embed         │  │ │
│       │                         │  │  - VectorStore   │  │ │
│       ▼                         │  └──────────────────┘  │ │
│  ┌──────────────┐               └────────────────────────┘ │
│  │  PostgreSQL  │                         ▲               │
│  │  (Reports)   │                         │ OTLP / gRPC   │
│  └──────────────┘                         │               │
│                                           │               │
│  ┌──────────────┐                  ┌──────────────────┐   │
│  │   Collector  │──OTLP (Push)────►│  MeerkatLogs     │   │
│  │   Server     │                  │  Server          │   │
│  └──────────────┘                  └──────────────────┘   │
│       ▲                                                    │
│       │ OTLP                                               │
│  ┌──────────────┐                                          │
│  │  Apps / SDK  │                                          │
│  └──────────────┘                                          │
└─────────────────────────────────────────────────────────────┘
```

### Services

| Service | Protocol | Responsibility |
|---------|----------|----------------|
| **Analyzer** | HTTP (REST) | AI analysis, report management, scheduling |
| **MeerkatLogs** | gRPC + OTLP | Log ingestion, semantic search, vector storage |
| **Collector** | OTLP (gRPC) | Log collection gateway to MeerkatLogs |

### Log Ingestion Flow

```
App ──OTLP──→ Collector ──OTLP──→ MeerkatLogs ──embed──→ VectorStore (Milvus/Qdrant)
```

### Analysis Flow

```
Client ──HTTP──→ Analyzer ──gRPC──→ MeerkatLogs (search_logs)
                     │
                     └──→ LLM + Tools (prometheus, loki, victorialogs)
                     │
                     └──→ PostgreSQL (Reports)
```

## Quick Start

### Prerequisites

- PostgreSQL 14+
- Milvus or Qdrant (vector store)
- OpenAI API key (or compatible provider)

### Configuration

```bash
# Copy example config
cp config.example.yaml config.yaml

# Edit config.yaml with your settings
```

```yaml
app:
  name: meerkat
  env: development

http:
  host: 0.0.0.0
  port: 8080

store:
  driver: postgres
  host: localhost
  port: 5432
  name: meerkat
  user: meerkat
  password: meerkat

meerkat_logs:
  enabled: true
  address: ":50051"
  otlp_bind_addr: ":4317"
  filter_mode: template  # all | severity | template
  min_severity: info
  retention: 72h

embedder:
  provider: openai
  api_key: ${OPENAI_API_KEY}
  model: text-embedding-3-small

vector_store:
  milvus:
    address: localhost:19530
    collection: logs
    dimension: 1536
```

### Running Services

```bash
# Run database migrations
go run ./cmd/meerkat-server analyzer migrate apply

# Start MeerkatLogs server (log ingestion + search)
go run ./cmd/meerkat-server logs serve

# Start Analyzer server (HTTP API + AI analysis)
go run ./cmd/meerkat-server analyzer serve

# Start Collector server (OTLP log collection) - optional
go run ./cmd/meerkat-server collector serve
```

### CLI

```bash
# Trigger manual inspection
meerkat inspect -q "Check for error spikes in the last hour"

# List reports
meerkat report list

# Get specific report
meerkat report get <id>
```

## Configuration Validation

All services validate configuration at startup:

```go
if err := cfg.Validate(); err != nil {
    return fmt.Errorf("validate config: %w", err)
}
```

Checks include:
- Port ranges (0-65535)
- Required database fields
- Analyzer provider (openai | anthropic)
- Vector store driver (milvus | qdrant)
- Filter mode (all | severity | template)

## Filtering Modes

MeerkatLogs supports three log filtering modes during ingestion:

| Mode | Behavior |
|------|----------|
| **all** | All logs are vectorized (no filtering) |
| **severity** | Only logs with severity ≥ `min_severity` are vectorized |
| **template** | Drain algorithm extracts templates, duplicates are deduplicated (default) |

## Metrics

MeerkatLogs exposes Prometheus metrics on `:9090/metrics`:

- `meerkatlogs_ingest_total` — Total ingested logs
- `meerkatlogs_ingest_deduplicated_total` — Total deduplicated logs
- `meerkatlogs_search_duration_seconds` — Search latency histogram
- `meerkatlogs_search_total` — Total search requests

## Development

```bash
# Generate all code (ent, proto, ogen, mocks)
make gen

# Build binaries
make build

# Run unit tests
make test

# Run integration tests
make test-integration

# Run e2e tests (requires Docker)
make test-e2e

# Run linter
make lint
```

## Deployment

### Docker

```bash
docker build -f build/docker/server.Dockerfile -t meerkat-server .
```

### Kubernetes (Helm)

```bash
helm install meerkat ./deployment/charts/meerkat \
  --set logs.enabled=true \
  --set analyzer.enabled=true
```

## License

Proprietary
