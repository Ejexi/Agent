.PHONY: build clean test

build:
	go build -o duckops.exe ./cmd/duckops

clean:
	rm -f duckops.exe

test:
	go test ./...

# Build with version info
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "none")
DATE    ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ 2>/dev/null || echo "unknown")

release:
	go build -ldflags "-X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.date=$(DATE)" -o duckops.exe ./cmd/duckops
