# Assignment Pull Request Creator

.PHONY: build test clean lint fmt vet run help coverage coverage-html test-unit test-integration bench

# Build configurations
BINARY_NAME=assignment-pr-creator
BINARY_PATH=./bin/$(BINARY_NAME)
CMD_PATH=./cmd/assignment-pr-creator

# Test and coverage configurations
TEST_COVERAGE_DIR=coverage
TEST_COVERAGE_FILE=$(TEST_COVERAGE_DIR)/coverage.out
TEST_HTML_FILE=$(TEST_COVERAGE_DIR)/coverage.html

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod
GOFMT=$(GOCMD) fmt

# Test flags
TEST_FLAGS=-v -race
BENCH_FLAGS=-bench=. -benchmem

# Default target
all: check

## build: Build the binary
build: clean
	@echo "Building $(BINARY_NAME)..."
	@mkdir -p bin
	$(GOBUILD) -o $(BINARY_PATH) $(CMD_PATH)
	@echo "✅ Binary built at $(BINARY_PATH)"

## build-hook: Build the post-checkout hook
build-hook:
	@echo "Building post-checkout hook..."
	@mkdir -p bin
	$(GOBUILD) -o ./bin/post-checkout ./cmd/githook
	@echo "✅ Post-checkout hook built at ./bin/post-checkout"

## install-hook: Install the post-checkout hook globally
install-hook: build-hook
	@echo "Installing post-checkout hook..."
	@mkdir -p ~/.githooks
	@cp ./bin/post-checkout ~/.githooks/post-checkout
	@chmod +x ~/.githooks/post-checkout
	@git config --global core.hooksPath ~/.githooks
	@echo "✅ Post-checkout hook installed at ~/.githooks/post-checkout"

## test: Run all tests
test:
	@echo "Running tests..."
	$(GOTEST) $(TEST_FLAGS) ./...

## test-unit: Run unit tests only (internal packages)
test-unit:
	@echo "Running unit tests..."
	$(GOTEST) $(TEST_FLAGS) ./internal/...

## test-integration: Run integration tests
test-integration:
	@echo "Running integration tests..."
	$(GOTEST) $(TEST_FLAGS) ./cmd/assignment-pr-creator

## test-creator: Run creator package tests
test-creator:
	@echo "Running creator tests..."
	$(GOTEST) $(TEST_FLAGS) ./internal/creator

## test-git: Run git package tests
test-git:
	@echo "Running git tests..."
	$(GOTEST) $(TEST_FLAGS) ./internal/git

## test-github: Run github package tests
test-github:
	@echo "Running github tests..."
	$(GOTEST) $(TEST_FLAGS) ./internal/github

## test-testutil: Run testutil package tests
test-testutil:
	@echo "Running testutil tests..."
	$(GOTEST) $(TEST_FLAGS) ./internal/testutil

## test-short: Run short tests (skip long-running tests)
test-short:
	@echo "Running short tests..."
	$(GOTEST) $(TEST_FLAGS) -short ./...

## bench: Run benchmarks
bench:
	@echo "Running benchmarks..."
	$(GOTEST) $(BENCH_FLAGS) ./...

## clean: Clean build artifacts
clean:
	@echo "Cleaning..."
	$(GOCLEAN)
	@rm -rf ./bin
	@rm -rf $(TEST_COVERAGE_DIR)

## coverage: Run tests with coverage
coverage:
	@echo "Running tests with coverage..."
	@mkdir -p $(TEST_COVERAGE_DIR)
	$(GOTEST) $(TEST_FLAGS) -coverprofile=$(TEST_COVERAGE_FILE) ./...
	@echo "Coverage summary:"
	$(GOCMD) tool cover -func=$(TEST_COVERAGE_FILE) | grep -E "(total|\.go:)" | tail -10

## coverage-html: Generate HTML coverage report
coverage-html:
	@echo "Running tests with coverage for HTML report..."
	@mkdir -p $(TEST_COVERAGE_DIR)
	$(GOTEST) $(TEST_FLAGS) -coverprofile=$(TEST_COVERAGE_FILE) ./...
	@echo "Generating HTML coverage report..."
	$(GOCMD) tool cover -html=$(TEST_COVERAGE_FILE) -o=$(TEST_HTML_FILE)
	@echo "✅ Coverage report generated: $(TEST_HTML_FILE)"

## coverage-show: Show detailed coverage in terminal
coverage-show: $(TEST_COVERAGE_DIR)
	@echo "Running tests with coverage for detailed report..."
	$(GOTEST) $(TEST_FLAGS) -coverprofile=$(TEST_COVERAGE_FILE) ./...
	@echo "Detailed coverage report:"
	$(GOCMD) tool cover -func=$(TEST_COVERAGE_FILE)

## lint: Run linter (auto-installs golangci-lint if needed)
lint:
	@echo "Running linter..."
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest; \
	echo "✅ golangci-lint installed, running linter..."; \
	golangci-lint run -v; \

## fmt: Format Go code
fmt:
	@echo "Formatting code..."
	$(GOFMT) ./...

## vet: Run go vet
vet:
	@echo "Running go vet..."
	$(GOCMD) vet ./...

## deps: Download dependencies
deps:
	@echo "Downloading dependencies..."
	$(GOMOD) download
	$(GOMOD) tidy

## run: Build and run with dry-run mode
run: build
	@echo "Running $(BINARY_NAME) in dry-run mode..."
	@GITHUB_TOKEN=dummy \
	 GITHUB_REPOSITORY=owner/repo \
	 DRY_RUN=true \
	 $(BINARY_PATH)

## run-live: Build and run in live mode (requires real environment variables)
run-live: build
	@echo "Running $(BINARY_NAME) in live mode..."
	@if [ -z "$(GITHUB_TOKEN)" ] || [ -z "$(GITHUB_REPOSITORY)" ]; then \
		echo "❌ Error: GITHUB_TOKEN and GITHUB_REPOSITORY environment variables are required for live mode"; \
		echo "Usage: GITHUB_TOKEN=your_token GITHUB_REPOSITORY=owner/repo make run-live"; \
		exit 1; \
	fi
	$(BINARY_PATH)

## install: Install the binary to GOPATH/bin
install:
	@echo "Installing $(BINARY_NAME)..."
	$(GOCMD) install $(CMD_PATH)

## docker-build: Build Docker image (if Dockerfile exists)
docker-build:
	@if [ -f Dockerfile ]; then \
		echo "Building Docker image..."; \
		docker build -t $(BINARY_NAME) .; \
	else \
		echo "⚠️  Dockerfile not found"; \
	fi

## check: Run all checks (fmt, vet, lint, test)
check: fmt vet lint test coverage
	@echo "✅ All checks passed"

## check-quick: Run quick checks (fmt, vet, test-short)
check-quick: fmt vet test-short
	@echo "✅ Quick checks passed"

## check-ci: Run CI checks (vet, test)
check-ci: vet test
	@echo "✅ CI checks passed"

## ci: Run CI pipeline (for GitHub Actions)
ci: deps fmt vet lint coverage
	@echo "✅ CI pipeline completed"

## help: Show this help message
help:
	@echo "Available targets:"
	@echo ""
	@echo "Build commands:"
	@echo "  build         Build the binary"
	@echo "  install       Install the binary to GOPATH/bin"
	@echo "  clean         Clean build artifacts"
	@echo ""
	@echo "Test commands:"
	@echo "  test          Run all tests"
	@echo "  test-unit     Run unit tests only (internal packages)"
	@echo "  test-integration Run integration tests"
	@echo "  test-creator  Run creator package tests"
	@echo "  test-git      Run git package tests"
	@echo "  test-github   Run github package tests"
	@echo "  test-testutil Run testutil package tests"
	@echo "  test-short    Run short tests (skip long-running tests)"
	@echo "  bench         Run benchmarks"
	@echo ""
	@echo "Coverage commands:"
	@echo "  coverage      Run tests with coverage"
	@echo "  coverage-html Generate HTML coverage report"
	@echo "  coverage-show Show detailed coverage in terminal"
	@echo ""
	@echo "Code quality commands:"
	@echo "  fmt           Format Go code"
	@echo "  vet           Run go vet"
	@echo "  lint          Run linter (auto-installs golangci-lint if needed)"
	@echo "  check         Run all checks (fmt, vet, lint, test)"
	@echo "  check-quick   Run quick checks (fmt, vet, test-short)"
	@echo "  check-ci      Run CI checks (vet, test)"
	@echo ""
	@echo "Dependency commands:"
	@echo "  deps          Download dependencies"
	@echo ""
	@echo "Run commands:"
	@echo "  run           Build and run with dry-run mode"
	@echo "  run-live      Build and run in live mode (requires real environment variables)"
	@echo ""
	@echo "Docker commands:"
	@echo "  docker-build  Build Docker image (if Dockerfile exists)"
	@echo ""
	@echo "CI/CD commands:"
	@echo "  ci            Run CI pipeline (for GitHub Actions)"

# Version information
version:
	@echo "Go version: $(shell $(GOCMD) version)"
	@echo "Module: $(shell $(GOMOD) list -m)"