# ==========================================
# Phase 1: Go Builder
# ==========================================
FROM golang:1.24-alpine AS builder

RUN apk --no-cache add ca-certificates tzdata git

WORKDIR /src
COPY shared/ ./shared/
COPY agent/go.mod agent/go.sum ./agent/
WORKDIR /src/agent
RUN go mod download
WORKDIR /src
COPY agent/ ./agent/
WORKDIR /src/agent
RUN go mod tidy

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -v -ldflags="-w -s" -o /go/bin/agent ./cmd/agent-cli

# ==========================================
# Phase 3: Final Runtime Image
# ==========================================
FROM debian:bookworm-slim

# ── Non-root user ───────────────────────────────────

RUN groupadd -r duckops && \
    useradd -r -g duckops -m -s /bin/bash duckops && \
    mkdir -p /app /workspace /tmp/scans && \
    chown -R duckops:duckops /app /workspace /tmp/scans

# ── System Dependencies ────────────────────────────────────────────────────────
RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates tzdata curl wget git jq \
    && apt-get clean && rm -rf /var/lib/apt/lists/*

# ── Copy compiled Go agent ────────────────────────────────────────────────────
WORKDIR /app
COPY --from=builder /go/bin/agent .


# ── Environment ───────────────────────────────────────────────────────────────
ENV LANG=en_US.UTF-8
ENV TZ=UTC

# ── Switch to non-root ────────────────────────────────────────────────────────
USER duckops
WORKDIR /workspace

ENTRYPOINT ["/app/agent"]