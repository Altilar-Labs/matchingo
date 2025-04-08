SHELL := /bin/bash

.PHONY: test imports fix clean build proto build-all run-server run-client

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
