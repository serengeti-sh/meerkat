# Meerkat

AI-powered observability agent that watches your infrastructure like a meerkat on sentinel duty.

## Overview

Meerkat is an AI-driven observability platform consisting of **three independently deployable services**:

- **Analyzer**: HTTP API server that orchestrates AI analysis, report management, and scheduling
- **MeerkatLogs**: gRPC + OTLP server for log ingestion, semantic search, and vector storage
- **Collector**: OTLP gateway that receives log exports and forwards them to MeerkatLogs

## Architecture

```mermaid
graph TB
    subgraph "Data Sources"
        App[Applications / SDKs]
    end

    subgraph "Meerkat Platform"
        Collector[Collector<br/>OTLP Receiver :4317]
        MeerkatLogs[MeerkatLogs<br/>gRPC :50051 / OTLP :4317]
        Analyzer[Analyzer<br/>HTTP API :8080]
        PostgreSQL[(PostgreSQL<br/>Reports)]
        VectorStore[(Vector Store<br/>Milvus / Qdrant)]
    end

    subgraph "AI Engine"
        LLM[LLM Provider<br/>OpenAI / Anthropic]
        Tools[Tools<br/>Prometheus / Loki / VictoriaLogs]
    end

    App -->|OTLP Push| Collector
    Collector -->|OTLP / gRPC| MeerkatLogs
    MeerkatLogs -->|Embed & Store| VectorStore
    Analyzer -->|gRPC Search| MeerkatLogs
    Analyzer -->|SQL| PostgreSQL
    Analyzer -->|HTTP| LLM
    Analyzer -->|Query| Tools
```

### Log Ingestion Flow

```mermaid
sequenceDiagram
    participant App as Application
    participant Collector as Collector (:4317)
    participant ML as MeerkatLogs (:50051)
    participant VS as Vector Store

    App->>Collector: OTLP Log Export
    Collector->>Collector: Batch & Buffer
    Collector->>ML: gRPC Ingest()
    ML->>ML: Filter (severity/template)
    ML->>ML: Extract Template (Drain)
    ML->>ML: Embed (OpenAI)
    ML->>VS: Store Vector + Metadata
    ML-->>Collector: IngestResult
```

### Analysis Flow

```mermaid
sequenceDiagram
    participant Client as Client / Webhook
    participant Analyzer as Analyzer (:8080)
    participant ML as MeerkatLogs (:50051)
    participant LLM as LLM Provider
    participant DB as PostgreSQL

    Client->>Analyzer: POST /v1/inspect or /v1/webhook
    Analyzer->>ML: gRPC GetContext(service, time range)
    ML-->>Analyzer: Log Context
    Analyzer->>LLM: Analysis Prompt + Tools + Context
    LLM-->>Analyzer: Analysis Result
    Analyzer->>DB: Persist Report
    Analyzer-->>Client: Report ID
```

## Services

| Service | Protocol | Default Port | Responsibility |
|---------|----------|--------------|----------------|
| **Analyzer** | HTTP (REST) | 8080 | AI analysis, report management, scheduling, webhook reception |
| **MeerkatLogs** | gRPC + OTLP | 50051 (gRPC), 4317 (OTLP), 51051 (metrics) | Log ingestion, template extraction, semantic search, vector storage |
| **Collector** | OTLP (gRPC) | 4317 | Log collection gateway, batching, forwarding to MeerkatLogs |

## Project Structure

```text
.
├── api/                          # API definitions
│   ├── openapi.yaml              # OpenAPI 3.0 spec
│   ├── paths/                    # OpenAPI path definitions
│   ├── schemas/                  # OpenAPI schema definitions
│   └── proto/meerkatlogs/v1/     # Protobuf service definitions
├── build/docker/                 # Dockerfiles
├── cmd/                          # Entry points
│   ├── meerkat/                  # CLI client
│   └── meerkat-server/           # Server binaries
│       ├── analyzer/             # Analyzer server commands
│       ├── collector/            # Collector server commands
│       └── logs/                 # MeerkatLogs server commands
├── deployment/
│   └── charts/meerkat/           # Helm chart
│       ├── templates/            # K8s manifests
│       └── values.yaml           # Default configuration
├── internal/                     # Private packages
│   ├── analyzer/                 # AI analysis engine
│   ├── collector/                # OTLP receiver & batcher
│   ├── config/                   # Configuration loading & validation
│   ├── embedder/                 # Text embedding (OpenAI)
│   ├── ent/                      # Ent ORM generated code
│   ├── httphandler/              # HTTP handlers (analyzer API)
│   ├── inspector/                # Report lifecycle & worker pool
│   ├── logsclient/               # gRPC client for MeerkatLogs
│   ├── logstream/                # Streaming log processor
│   ├── meerkatlogs/              # Log ingestion pipeline
│   ├── meerkatlogspb/            # Generated protobuf code
│   ├── metrics/                  # Prometheus metrics
│   ├── report/                   # Report domain & repository
│   ├── reporter/                 # Notification service (Slack)
│   ├── scheduler/                # Scheduled analysis jobs
│   ├── tool/                     # Observability tool integrations
│   └── vectorstore/              # Vector store clients (Milvus, Qdrant)
├── pkg/api/                      # Generated OpenAPI client/server
├── test/
│   ├── deploy/                   # Helm deployment tests (Kind)
│   ├── e2e/                      # End-to-end tests
│   └── integration/              # Integration tests
├── Makefile                      # Build automation
├── config.example.yaml           # Example configuration
└── go.mod                        # Go module definition
```

## Quick Start

### Prerequisites

- Go 1.26+
- PostgreSQL 14+
- Milvus or Qdrant (vector store)
- OpenAI API key (or compatible provider)
- (Optional) Helm 3+ for Kubernetes deployment

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
# Build CLI
go build -o meerkat ./cmd/meerkat

# Trigger manual inspection
./meerkat inspect -q "Check for error spikes in the last hour"

# List reports
./meerkat report list

# Get specific report
./meerkat report get <id>
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
- Analyzer provider (`openai` | `anthropic`)
- Vector store driver (`milvus` | `qdrant`)
- Filter mode (`all` | `severity` | `template`)
- MeerkatLogs address when enabled

## Filtering Modes

MeerkatLogs supports three log filtering modes during ingestion:

| Mode | Behavior |
|------|----------|
| **all** | All logs are vectorized (no filtering) |
| **severity** | Only logs with severity ≥ `min_severity` are vectorized |
| **template** | Drain algorithm extracts templates; duplicates are deduplicated (default) |

## Metrics

MeerkatLogs exposes Prometheus metrics on `:51051/metrics`:

- `meerkatlogs_ingest_total` — Total ingested logs
- `meerkatlogs_ingest_deduplicated_total` — Total deduplicated logs
- `meerkatlogs_search_duration_seconds` — Search latency histogram
- `meerkatlogs_search_total` — Total search requests
- `meerkatlogs_embed_duration_seconds` — Embedding latency histogram

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

# Format code
make fmt
```

## Deployment

### Docker

```bash
docker build -f build/docker/server.Dockerfile -t meerkat-server .
```

### Kubernetes (Helm)

```bash
# Install with default values
helm install meerkat ./deployment/charts/meerkat \
  --set logs.enabled=true \
  --set analyzer.enabled=true

# Install with custom values
helm install meerkat ./deployment/charts/meerkat \
  --values custom-values.yaml
```

### Service Ports

| Service | Port | Protocol | Purpose |
|---------|------|----------|---------|
| Analyzer | 8080 | HTTP | REST API |
| MeerkatLogs | 50051 | gRPC | Search/Ingest/GetContext |
| MeerkatLogs | 4317 | OTLP/gRPC | Log ingestion |
| MeerkatLogs | 51051 | HTTP | Metrics & Health |
| Collector | 4317 | OTLP/gRPC | Log collection |

## Security

- **TLS**: Analyzer HTTP server supports TLS via `http.tls.cert_file` and `http.tls.key_file`
- **gRPC TLS**: `logsclient` supports `WithTransportCredentials()` for secure inter-service communication
- **API Keys**: Embedder and LLM provider API keys are loaded from environment variables (never commit to VCS)
- **Database**: PostgreSQL connection uses SSL mode configuration

## License

Proprietary
