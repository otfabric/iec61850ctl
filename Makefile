.PHONY: help all run fmt lint lint-ci vet test coverage cover check build build-nocheck build-all release-all install clean

help: ## This help
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z0-9_-]+:.*?## / {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}' $(MAKEFILE_LIST)

.DEFAULT_GOAL := help

# Ignore parent-directory go.work files so this module builds standalone.
export GOWORK := off
export CGO_ENABLED := 0

APP_NAME       = iec61850ctl
APP_SRC        = main.go
ARCHS          = linux/amd64 linux/arm64 linux/arm/v7 darwin/amd64 darwin/arm64 windows/amd64 windows/arm64
VERSION       := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
TAG           := $(shell git tag --points-at HEAD 2>/dev/null | head -1)
COMMIT        := $(shell git rev-parse --short HEAD 2>/dev/null || echo "none")
BUILD_DATE    := $(shell date -u '+%Y-%m-%dT%H:%M:%SZ')
LDFLAGS       := -ldflags "-s -w \
  -X 'main.version=$(VERSION)' \
  -X 'main.tag=$(TAG)' \
  -X 'main.commit=$(COMMIT)' \
  -X 'main.buildDate=$(BUILD_DATE)'"
BIN_DIR       := bin
BIN           := $(BIN_DIR)/$(APP_NAME)
RELEASE_DIR   := release

all: test build-nocheck build-all ## Test and build the application

run: ## Run the application
	@echo "Running $(APP_NAME)"
	@go run $(LDFLAGS) $(APP_SRC)

fmt: ## Format Go code (gofmt + goimports via golangci-lint)
	@echo "Running go fmt"
	@go fmt ./...
	@echo "Running golangci-lint fmt"
	@golangci-lint fmt ./...

lint: ## Run staticcheck
	@echo "Running staticcheck"
	@staticcheck ./...

lint-ci: ## Run golangci-lint (see .golangci.yml; unusedwrite enabled)
	@echo "Running golangci-lint"
	@golangci-lint run ./...

vet: ## Run go vet on project packages
	@echo "Running go vet"
	@go vet ./...

# Library packages under coverage (CLI cmd/ wiring excluded; see .codecov.yml).
COVERPKGS := ./pkg/... ./internal/...

test: ## Run unit tests with race detector
	@echo "Running unit tests (race detector)"
	@CGO_ENABLED=1 go test -count=1 -race ./...

coverage: ## Run tests with coverage for pkg/ and internal/ (writes coverage.out)
	@echo "Running coverage ($(COVERPKGS))"
	@CGO_ENABLED=1 go test -count=1 -race -coverprofile=coverage.out -covermode=atomic $(COVERPKGS)
	@go tool cover -func=coverage.out | tail -1

cover: coverage ## Open coverage report in browser
	@echo "Opening coverage report"
	@go tool cover -html=coverage.out

check: fmt lint lint-ci vet test coverage ## Run all checks (format, lint, vet, test, coverage)

build: check ## Build the application (runs full check first)
	@$(MAKE) build-nocheck

build-nocheck: ## Build the application without running checks
	@echo "Building $(BIN)"
	@mkdir -p $(BIN_DIR)
	@go build $(LDFLAGS) -o $(BIN) $(APP_SRC)

build-all: ## Build the application for all architectures
	@mkdir -p $(RELEASE_DIR)
	@for arch in $(ARCHS); do \
		os=$${arch%%/*}; \
		rest=$${arch#*/}; \
		cpu=$${rest%%/*}; \
		variant=$${rest#*/}; \
		ext=""; \
		if [ "$$os" = "windows" ]; then ext=".exe"; fi; \
		if [ "$$cpu" = "arm" ] && [ "$$variant" = "v7" ]; then \
			out=$(APP_NAME)-$(VERSION)-$$os-armv7$$ext; \
			echo "Building $$out..."; \
			GOOS=$$os GOARCH=$$cpu GOARM=7 go build $(LDFLAGS) -o $(RELEASE_DIR)/$$out $(APP_SRC); \
		else \
			out=$(APP_NAME)-$(VERSION)-$$os-$$cpu$$ext; \
			echo "Building $$out..."; \
			GOOS=$$os GOARCH=$$cpu go build $(LDFLAGS) -o $(RELEASE_DIR)/$$out $(APP_SRC); \
		fi \
	done

release-all: build-all ## Package the build binaries into tar.gz archives for all architectures
	@mkdir -p $(RELEASE_DIR)
	@for arch in $(ARCHS); do \
		os=$${arch%%/*}; \
		rest=$${arch#*/}; \
		cpu=$${rest%%/*}; \
		variant=$${rest#*/}; \
		ext=""; \
		if [ "$$os" = "windows" ]; then ext=".exe"; fi; \
		if [ "$$cpu" = "arm" ] && [ "$$variant" = "v7" ]; then \
			bin=$(APP_NAME)-$(VERSION)-$$os-armv7$$ext; \
		else \
			bin=$(APP_NAME)-$(VERSION)-$$os-$$cpu$$ext; \
		fi; \
		echo "Packaging $$bin.tar.gz..."; \
		tar czf $(RELEASE_DIR)/$$bin.tar.gz -C $(RELEASE_DIR) $$bin; \
	done

install: build ## Install the built binary to /usr/local/bin
	@echo "Installing $(APP_NAME) to /usr/local/bin"
	@sudo install -m 0755 $(BIN) /usr/local/bin/$(APP_NAME)

clean: ## Clean the build artifacts
	@echo "Cleaning build artifacts"
	@rm -rf $(BIN_DIR)
	@rm -rf $(RELEASE_DIR)
	@rm -f coverage.out
