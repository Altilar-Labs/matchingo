SHELL := /bin/bash

.PHONY: test test-unit test-integration test-redis test-stop-orders imports fix clean build proto build-all run-server run-client test-deps-up test-deps-down bench bench-memory bench-redis bench-verbose

# Test targets
test: test-unit test-integration

test-unit: imports fix
	@echo "Running unit tests..."
	go test -v -race -coverprofile=coverage.out ./pkg/...

test-integration:
	@echo "Starting dependencies for integration tests..."
	$(MAKE) test-deps-up
	@trap '$(MAKE) test-deps-down' EXIT; \
	echo "Running integration tests..."; \
	go test -v -race ./test/integration/... -run IntegrationV2

test-redis:
	@echo "Starting dependencies for Redis tests..."
	$(MAKE) test-deps-up
	@trap '$(MAKE) test-deps-down' EXIT; \
	echo "Running Redis integration tests..."; \
	go test -v -race ./test/integration/... -run RedisIntegration

test-stop-orders:
	@echo "Starting dependencies for stop order tests..."
	$(MAKE) test-deps-up
	@trap '$(MAKE) test-deps-down' EXIT; \
	echo "Running stop order tests with Docker containers..."; \
	go test -v ./test/integration/... -run 'TestIntegrationV2_.*StopLimit|TestIntegrationV2_.*StopLimitActivation'

# Development targets
demo-memory:
	go run cmd/examples/basic/main.go

demo-redis:
	go run cmd/examples/redis/main.go

# Benchmark targets
bench:
	go test -bench=. -benchmem -benchtime=1s ./pkg/...

bench-memory:
	go test -bench=Memory -benchmem -benchtime=1s ./pkg/backend/memory

bench-redis:
	go test -bench=Redis -benchmem -benchtime=1s ./pkg/backend/redis

bench-verbose:
	go test -v -bench=. -benchmem -benchtime=1s ./pkg/...

# Build targets
imports:
	goimports -w .

fix:
	gofmt -s -w .

clean:
	@echo "Cleaning..."
	@rm -rf bin/ build/
	@rm -f pkg/api/proto/orderbook/*.pb.go
	@echo "Clean complete."

proto:
	@echo "Generating protobuf code..."
	@mkdir -p pkg/api/proto/orderbook
	@protoc -I=. \
		--go_out=. --go-grpc_out=. \
		--go_opt=paths=source_relative \
		--go-grpc_opt=paths=source_relative \
		pkg/api/proto/orderbook.proto

build:
	@echo "Building server and client..."
	@mkdir -p bin
	@go build -o bin/orderbook-server cmd/server/main.go
	@go build -o bin/orderbook-client cmd/client/main.go
	@echo "Build complete. Binaries in ./bin/"

build-all: proto build

# Run targets
run-server:
	@echo "Running orderbook server..."
	@./bin/orderbook-server

run-client:
	@echo "Running orderbook client..."
	@./bin/orderbook-client --cmd=list

# Test dependency management
test-deps-up:
	@echo "Starting test dependencies (Redis)..."
	docker compose -f docker-compose.yml up -d --wait redis
	@echo "Test dependencies started."

test-deps-down:
	@echo "Stopping test dependencies (Redis)..."
	docker compose -f docker-compose.yml down
	@echo "Test dependencies stopped."

# Default target
default: build
