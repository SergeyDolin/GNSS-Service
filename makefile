.PHONY: test test-cover test-integration test-unit

test:
	go test ./... -v

test-cover:
	go test ./... -coverprofile=coverage.out
	go tool cover -html=coverage.out

test-unit:
	go test ./internal/storage/... ./internal/handlers/... ./cmd/rtklib/processor/... -v

test-integration:
	go test ./cmd/... -v

bench:
	go test -bench=. -benchmem ./...


lint:
	golangci-lint run

.PHONY: help
help:
	@echo "Available commands:"
	@echo "  make test              - Run all tests"
	@echo "  make test-cover        - Run tests with coverage"
	@echo "  make test-unit         - Run unit tests only"
	@echo "  make test-integration  - Run integration tests"
	@echo "  make bench             - Run benchmarks"
	@echo "  make db-test-create    - Create test database"
	@echo "  make db-test-drop      - Drop test database"
	@echo "  make db-test-reset     - Reset test database"
	@echo "  make lint              - Run linter"