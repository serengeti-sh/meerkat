# Contributing to Meerkat

Thank you for your interest in contributing to Meerkat! This document provides guidelines and workflows for development.

## Development Environment

### Prerequisites

- **Go**: 1.26 or later
- **PostgreSQL**: 14 or later (for local development)
- **Vector Store**: Milvus or Qdrant (for Vectors functionality)
- **Protocol Buffers Compiler**: `protoc` 3.21+ (for code generation)
- **Node.js**: 18+ (for OpenAPI bundling)
- **Docker**: For e2e and deployment tests
- **Kind**: For Helm deployment tests

### Recommended Tools

```bash
# Go tools
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
go install entgo.io/ent/cmd/ent@latest

# Node.js tools (for OpenAPI)
npm install -g @apidevtools/swagger-cli

# Mock generation
go install github.com/vektra/mockery/v2@latest

# Linting
make golangci-lint  # Installs automatically
```

## Getting Started

1. **Fork and clone** the repository:

   ```bash
   git clone git@github.com:your-username/meerkat.git
   cd meerkat
   ```

2. **Install dependencies**:

   ```bash
   go mod download
   ```

3. **Copy configuration**:

   ```bash
   cp config.example.yaml config.yaml
   # Edit config.yaml with your local settings
   ```

4. **Verify setup**:

   ```bash
   make build
   make test
   ```

## Code Generation

Meerkat uses code generation for multiple components. After modifying schemas or contracts, regenerate code:

```bash
# Generate all code
make gen

# Individual generators
make ent-gen          # Ent ORM from schema
make proto            # Go from protobuf definitions
make ogen             # HTTP handlers from OpenAPI spec
make mock             # Mocks from interfaces
```

### When to Regenerate

| Modified | Command |
|----------|---------|
| `internal/ent/schema/*.go` | `make ent-gen` |
| `api/proto/**/*.proto` | `make proto` |
| `api/openapi.yaml` or `api/paths/*.yaml` | `make ogen` |
| Interface definitions in any package | `make mock` |

## Testing

### Test Levels

```bash
# Unit tests (fast, no external dependencies)
make test

# Integration tests (in-memory DI, PostgreSQL container)
make test-integration

# End-to-end tests (real compiled binary, containers)
make test-e2e

# Deployment tests (Kind cluster, Helm chart)
make test-deploy
```

### Writing Tests

- **Unit tests**: Place alongside source files (`*_test.go`)
- **Integration tests**: Use `test/integration/`
- **E2E tests**: Use `test/e2e/`
- **Mocks**: Generate with mockery, place in `mocks/` subdirectories

### Test Conventions

- Use `testify/assert` and `testify/require`
- Table-driven tests for validation logic
- Mock external services (LLM, datasources)
- Clean up resources in `t.Cleanup()`

## Code Style

### Linting and Formatting

```bash
# Run linter (golangci-lint)
make lint

# Auto-fix issues
make fmt
```

### Go Conventions

- Follow standard [Effective Go](https://go.dev/doc/effective_go) guidelines
- Use `gofmt` formatting (enforced by `make fmt`)
- Explicit error handling; never ignore errors
- Consumer-defined interfaces (Go best practice)
- Concrete return types, interface parameters

### Naming

- **Packages**: Short, lowercase, no underscores (`vectors`, `vectorsclient`, `inspect`)
- **Interfaces**: Noun describing capability (`Service`, `Store`, `Client`)
- **Implementations**: Descriptive (`service`, `milvusStore`)
- **Error messages**: Lowercase, no punctuation at end

## Commit Conventions

We follow [Conventional Commits](https://www.conventionalcommits.org/):

```text
<type>[<scope>]: <description>

[optional body]

[optional footer]
```

### Types

- `feat`: New feature
- `fix`: Bug fix
- `docs`: Documentation only
- `style`: Code style (formatting, no logic change)
- `refactor`: Code restructuring
- `test`: Adding or updating tests
- `chore`: Build process, dependencies, etc.

### Examples

```text
feat(vectors): add severity-based filtering

fix(vectors): retry failed embed calls up to 3 times

docs(readme): update architecture diagram with mermaid

refactor(config): rename vectors package to vectors
```

## Pull Request Process

1. **Branch**: Create a feature branch from `main`

   ```bash
   git checkout -b feat/your-feature-name
   ```

2. **Develop**: Make changes with tests

3. **Verify**:

   ```bash
   make build
   make test
   make lint
   ```

4. **Commit**: Use conventional commit format

5. **Push** and create PR with:
   - Clear description of changes
   - Link to related issues
   - Screenshots/diagrams for UI/architecture changes

6. **Review**: Address review feedback

7. **Merge**: Maintainers will merge after approval

## Architecture Decisions

### Package Boundaries

- **`internal/`**: Private implementation details
  - Domain packages (`analyzer`, `inspect`, `vectors`)
  - Infrastructure (`embed`, `vectorstore`, `discovery`)
  - Transport (`httphandler`, `vectorsclient`)
- **`pkg/api/`**: Generated public API code
- **`cmd/`**: Executable entry points

### Dependency Direction

```text
Analyzer -> vectorsclient -> vectorspb
         -> inspect -> analyzer, report, notify
         -> schedule -> inspect

Vectors -> vectors (domain service)
        -> embed, vectorstore
```

### Key Design Principles

1. **Push-only ingestion**: OTLP push model; no log pulling
2. **Service separation**: Analyzer never accesses vectorstore directly
3. **gRPC for inter-service**: All analyzer->vectors communication via gRPC
4. **Config validation**: All services validate config on startup
5. **Explicit lifecycle**: `Start()`/`Stop()` methods for background services

## Troubleshooting

### Common Issues

**Build failures after schema changes**:

```bash
make clean
make gen
make build
```

**Testcontainers timeout**:

- Increase Docker resources (CPU/memory)
- Set `TESTCONTAINERS_RYUK_DISABLED=true`

**Protobuf generation fails**:

```bash
# Ensure protoc plugins are in PATH
export PATH=$PATH:$(go env GOPATH)/bin
make proto
```

**Lint errors**:

```bash
make fmt  # Auto-fixes many issues
make lint # Check remaining
```

## Questions?

- Open an issue for bugs or feature requests
- Check existing issues and PRs before creating new ones
- Follow the code of conduct in all interactions
