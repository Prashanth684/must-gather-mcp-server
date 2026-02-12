.PHONY: help build clean test lint fmt tidy

BINARY_NAME=must-gather-mcp-server
OUTPUT_DIR=_output
BIN_DIR=$(OUTPUT_DIR)/bin
TOOLS_DIR=$(OUTPUT_DIR)/tools/bin

PACKAGE = $(shell go list -m)
GIT_COMMIT_HASH = $(shell git rev-parse HEAD 2>/dev/null || echo "unknown")
GIT_VERSION = $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_TIME = $(shell date -u '+%Y-%m-%dT%H:%M:%SZ')
LD_FLAGS = -s -w \
	-X '$(PACKAGE)/pkg/version.CommitHash=$(GIT_COMMIT_HASH)' \
	-X '$(PACKAGE)/pkg/version.Version=$(GIT_VERSION)' \
	-X '$(PACKAGE)/pkg/version.BuildTime=$(BUILD_TIME)' \
	-X '$(PACKAGE)/pkg/version.BinaryName=$(BINARY_NAME)'

GOLANGCI_LINT_VERSION=v1.63.4

OSES = darwin linux windows
ARCHS = amd64 arm64

help: ## Display this help
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

##@ Development

fmt: ## Format Go code
	go fmt ./...

tidy: ## Tidy Go modules
	go mod tidy

build: fmt tidy ## Build the binary
	@mkdir -p $(BIN_DIR)
	go build -ldflags "$(LD_FLAGS)" -o $(BIN_DIR)/$(BINARY_NAME) ./cmd/must-gather-mcp-server

build-all-platforms: fmt tidy ## Build for all platforms
	@mkdir -p $(BIN_DIR)
	$(foreach os,$(OSES),$(foreach arch,$(ARCHS), \
		GOOS=$(os) GOARCH=$(arch) go build -ldflags "$(LD_FLAGS)" \
		-o $(BIN_DIR)/$(BINARY_NAME)-$(os)-$(arch)$(if $(findstring windows,$(os)),.exe,) \
		./cmd/must-gather-mcp-server; \
	))

clean: ## Clean build artifacts
	rm -rf $(OUTPUT_DIR)

test: ## Run tests
	go test -v -race -coverprofile=coverage.out ./...

lint: $(TOOLS_DIR)/golangci-lint ## Run linter
	$(TOOLS_DIR)/golangci-lint run

##@ Tools

$(TOOLS_DIR)/golangci-lint: ## Install golangci-lint
	@mkdir -p $(TOOLS_DIR)
	@echo "Installing golangci-lint $(GOLANGCI_LINT_VERSION)..."
	@curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(TOOLS_DIR) $(GOLANGCI_LINT_VERSION)
