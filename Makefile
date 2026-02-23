.PHONY: build test clean run

BINARY_NAME=agent

build:
	@echo "Building Agent..."
	go build -o bin/$(BINARY_NAME) ./cmd/agent-cli

test:
	@echo "Running Agent Tests..."
	go test -v ./...

clean:
	@echo "Cleaning Agent binaries..."
	rm -rf bin/

run:
	go run ./cmd/agent-cli
