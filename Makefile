.PHONY: test test-unit test-integration test-e2e test-all coverage clean

# Run unit tests (default)
test: test-unit

# Run unit tests only (no integration tests)
test-unit:
	@echo "Running unit tests..."
	go test ./...

# Run integration tests (requires Docker)
test-integration:
	@echo "Running integration tests..."
	@echo "Note: Requires Docker to be running"
	go test -tags=integration -v ./internal/nats/

# Run E2E tests (requires Docker)
test-e2e:
	@echo "Running E2E tests..."
	@echo "Note: Requires Docker to be running"
	@docker info > /dev/null 2>&1 || (echo "Error: Docker is not running" && exit 1)
	go test -tags=e2e -v -timeout=10m ./e2e_test.go

# Run all tests (unit + integration + e2e)
test-all: test-unit test-integration test-e2e

# Run tests with coverage
coverage:
	@echo "Running tests with coverage..."
	go test -cover ./...

# Run tests with verbose output
test-verbose:
	go test -v ./...

# Clean test cache
clean:
	go clean -testcache

# Run tests in short mode (skip slow tests)
test-short:
	go test -short ./...

# Check if Docker is running (for integration tests)
check-docker:
	@docker info > /dev/null 2>&1 || (echo "Error: Docker is not running" && exit 1)
	@echo "Docker is running"
