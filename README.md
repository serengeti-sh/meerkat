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

## TODO

### Datasource 쿼리 레이어 리팩토링

현재 `QueryMetricsTool` / `QueryLogsTool`이 raw HTTP로 VM/VL API를 직접 호출하고 응답을 string으로 반환 중.
collector.go와 builtin_tools.go에 중복된 HTTP 로직이 있음.

개선 방향:

1. **datasource name 기반 조회** — tool parameter에서 `datasource_url` 제거, datasource name만 받아서 시스템이 URL 매핑. 현재 LLM이 URL을 직접 다루는 건 잘못된 설계
2. **공통 응답 타입 정의** — VM 응답 구조(`QueryResult`, `TimeSeries`, `LogResult` 등)를 파싱하는 타입 추가. 현재 raw string을 LLM에 전달
3. **collector.go + builtin_tools.go 중복 제거** — HTTP 호출 로직을 하나로 합치고 collector를 tool 내부에서 재사용
4. **VM API 확장** — `/api/v1/query_range` (범위 쿼리), `/api/v1/series` (시리즈 매칭) 지원. 현재 instant query만 가능
5. **공식 SDK 불필요** — Victoria Metrics/Logs는 HTTP API. Prometheus client library는 있지만 VM 직접 쿼리엔 표준 HTTP 클라이언트로 충분
