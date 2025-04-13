SHELL := /bin/bash

.PHONY: test imports fix clean build proto build-all run-server run-client test-deps-up test-deps-down test-integration test-redis test-stop-orders

test: imports fix
	go test ./pkg/...

test-v:
	go test ./pkg/... -v

demo-memory:
	go run cmd/examples/basic/main.go

demo-redis:
	go run cmd/examples/redis/main.go

bench:
	go test -bench=. -benchmem -benchtime=1s ./pkg/...

bench-memory:
	go test -bench=Memory -benchmem -benchtime=1s ./pkg/backend/memory

bench-redis:
	go test -bench=Redis -benchmem -benchtime=1s ./pkg/backend/redis

# Run a specific benchmark with verbose output and proper filtering
bench-verbose:
	go test -v -bench=. -benchmem -benchtime=1s ./pkg/...

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

run-server:
	@echo "Running orderbook server..."
	@./bin/orderbook-server

run-client:
	@echo "Running orderbook client..."
	@./bin/orderbook-client --cmd=list

# Testing Dependencies Management
test-deps-up:
	@echo "Starting test dependencies (Redis)..."
	docker compose -f docker-compose.yml up -d --wait redis-test
	@echo "Test dependencies started."

test-deps-down:
	@echo "Stopping test dependencies (Redis)..."
	docker compose -f docker-compose.yml down
	@echo "Test dependencies stopped."

# Add dependency management to main test target
test:
	@echo "Running tests..."
	go test -v -race -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out

test-integration:
	@echo "Starting dependencies for integration tests..."
	$(MAKE) test-deps-up
	@echo "Running integration tests..."
	go test -v -race ./pkg/server/... -run IntegrationV2 # Run only integration tests
	@echo "Stopping dependencies..."
	$(MAKE) test-deps-down

test-redis:
	@echo "Starting dependencies for Redis tests..."
	$(MAKE) test-deps-up
	@echo "Running Redis integration tests..."
	go test -v -race ./pkg/server/... -run RedisIntegration # Run only Redis integration tests
	@echo "Stopping dependencies..."
	$(MAKE) test-deps-down

test-stop-orders:
	@echo "Running stop order tests with Docker containers..."
	go test -v ./pkg/server/... -run 'TestIntegrationV2_.*StopLimit|TestIntegrationV2_.*StopLimitActivation'
	@echo "Tests completed."

# Default target
default: build
