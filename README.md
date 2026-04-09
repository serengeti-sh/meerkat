# Inspector

AI-powered observability agent that queries Victoria Metrics/Logs using an agentic AI loop.

## Architecture

```
Request (manual/webhook/scheduled)
  → Handler
    → Inspector Service (creates pending report, spawns goroutine)
      → Analyzer Service (agentic loop)
        → LLM Provider (OpenAI-compatible / Anthropic)
        → Tools (query_metrics, query_logs)
          → Victoria Metrics / Victoria Logs HTTP API
      → Reporter Service (Slack, Webhook channels)
```

## Configuration

```yaml
app:
  name: inspector
  env: development
  debug: false

http:
  host: 0.0.0.0
  port: 8080

store:
  driver: postgres
  path: postgresql://localhost:5432/inspector?sslmode=disable

datasources:
  - name: vm
    type: victoria-metrics
    url: http://localhost:8428
  - name: vl
    type: victoria-logs
    url: http://localhost:9428

analyzer:
  provider: openai          # openai (default), anthropic
  url: https://api.openai.com
  api_key: ${LLM_API_KEY}
  model: gpt-4o
  max_iterations: 10
  max_tokens: 4096
  temperature: 0.3
  system_prompt_file: ""    # optional: external system prompt file (falls back to built-in default)

scheduler:
  enabled: false
  jobs:
    - name: error-spike-check
      interval: "*/5 * * * *"
      metric_query: 'rate(http_errors_total[5m])'
      log_query: 'level:error'

reporter:
  channels:
    - type: slack
      webhook_url: https://hooks.slack.com/services/xxx
      min_severity: warning   # info, warning, critical
```

## API

| Method | Path | Description |
|--------|------|-------------|
| GET | /v1/health | Health check |
| POST | /v1/inspect | Start manual analysis |
| POST | /v1/webhook/{source} | Receive webhook alert |
| GET | /v1/reports | List reports |
| GET | /v1/reports/{id} | Get report by ID |
| GET | /v1/datasources | List configured datasources |
| GET | /v1/datasources/{name}/test | Test datasource connection |

## Development

```bash
make gen       # generate ent + ogen + mocks
make fmt       # format and lint fix
make mock      # regenerate mocks only
make test      # run tests
make build     # build binary
```

## Supported Datasources

| Type | Query Language | Supports |
|------|---------------|----------|
| `prometheus` | PromQL | Victoria Metrics, Prometheus, Thanos, Cortex, Mimir |
| `victoria-logs` | LogsQL | Victoria Logs |
| `loki` | LogQL | Grafana Loki |

## Architecture

```
Tool (query_metrics, query_logs)
  → ProviderRegistry.Get(name) → Provider
    → Provider.MetricsQuerier() / Provider.LogsQuerier()
      → Provider implementations (Prometheus, Victoria Logs, Loki)
```

Adding a new provider: implement `MetricsQuerier` and/or `LogsQuerier` interfaces, register in `newProvider()`.

## TODO

### API 확장

- `query_range` (범위 쿼리) 지원 — 현재 instant query만 가능
- `series` (시리즈 매칭) 엔드포인트 지원
