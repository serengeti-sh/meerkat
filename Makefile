# Tools
LOCALBIN ?= $(shell pwd)/bin
$(LOCALBIN):
	mkdir -p "$(LOCALBIN)"

GOLANGCI_LINT = $(LOCALBIN)/golangci-lint
GOLANGCI_LINT_VERSION ?= v2.1.0

# go-install-tool will 'go install' any package with custom target and name of binary, if it doesn't exist
define go-install-tool
@[ -f "$(1)-$(3)" ] && [ "$$(readlink "$(1)" 2>/dev/null)" = "$(1)-$(3)" ] || { \
set -e; \
package=$(2)@$(3) ;\
echo "Downloading $${package}" ;\
rm -f "$(1)" ;\
GOBIN="$(LOCALBIN)" go install $${package} ;\
mv "$(LOCALBIN)/$$(basename "$(1)")" "$(1)-$(3)" ;\
} ;\
ln -sf "$$(realpath "$(1)-$(3)")" "$(1)"
endef

.PHONY: golangci-lint
golangci-lint: $(GOLANGCI_LINT) ## Download golangci-lint locally if necessary.
$(GOLANGCI_LINT): $(LOCALBIN)
	$(call go-install-tool,$(GOLANGCI_LINT),github.com/golangci/golangci-lint/v2/cmd/golangci-lint,$(GOLANGCI_LINT_VERSION))

# Linting
.PHONY: lint
lint: golangci-lint ## Run golangci-lint linter
	"$(GOLANGCI_LINT)" run

.PHONY: fmt
fmt: golangci-lint ## Run go fmt and fix lint issues
	"$(GOLANGCI_LINT)" fmt
	"$(GOLANGCI_LINT)" run --fix

# Code Generation
.PHONY: ent-gen
ent-gen: ## Generate ent code
	go run -mod=mod entgo.io/ent/cmd/ent generate ./ent/schema

.PHONY: openapi-bundle
openapi-bundle: ## Bundle OpenAPI spec into single JSON file
	npx @apidevtools/swagger-cli bundle api/openapi.yaml --outfile api/openapi.bundled.json --type json

.PHONY: openapi-validate
openapi-validate: ## Validate bundled OpenAPI spec
	npx @apidevtools/swagger-cli validate api/openapi.bundled.json

.PHONY: openapi
openapi: openapi-bundle openapi-validate ## Bundle and validate OpenAPI spec

.PHONY: ogen
ogen: openapi-bundle ## Generate API code from OpenAPI spec
	@rm -f pkg/api/oas_*.go
	go tool ogen -config ogen.yaml -target ./pkg/api -package api api/openapi.bundled.json

.PHONY: mock
mock: ## Generate mocks
	@find ./internal -type d -name "mocks" -exec rm -rf {} + 2>/dev/null || true
	mockery

.PHONY: gen
gen: ent-gen ogen mock ## Generate all code (ent, ogen, mocks)

# Testing
.PHONY: test
test: ## Run unit tests
	go test -v $$(go list ./... | grep -v -e /mocks -e /test/integration -e /test/e2e -e /test/deploy) --coverprofile cover-unit.out

.PHONY: test-integration
test-integration: ## Run integration tests (uses in-memory DI, not real binary)
	go test -v ./test/integration/... -timeout 10m --coverprofile cover-integration.out

.PHONY: test-e2e
test-e2e: ## Run end-to-end tests (uses real built binary)
	go test -v ./test/e2e/... -timeout 10m --coverprofile cover-e2e.out

.PHONY: test-deploy
test-deploy: ## Run deployment integration test (requires Docker + Kind)
	go test -v ./test/deploy/... -timeout 15m

# Building
.PHONY: build
build: ## Build binaries
	go build -o bin/meerkat ./cmd/meerkat
	go build -o bin/meerkat-server ./cmd/meerkat-server

# All-in-one
.PHONY: all
all: gen build ## Generate all code and build

# Cleanup
.PHONY: clean
clean: ## Clean build artifacts
	rm -rf bin/
	rm -f api/openapi.bundled.json
	rm -f cover*.out

.PHONY: tidy
tidy: ## Tidy go modules
	go mod tidy

.PHONY: help
help: ## Show this help message
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'
