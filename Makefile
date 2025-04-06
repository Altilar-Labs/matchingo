SHELL := /bin/bash

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
	rm -rf build/

build:
	mkdir -p build
	go build -o build/basic cmd/examples/basic/main.go
	go build -o build/redis cmd/examples/redis/main.go
