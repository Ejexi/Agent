# ==========================================
# Phase 1: Build Stage
# ==========================================
FROM golang:1.23.6-alpine3.20 AS builder


# Install CA certificates to easily interact with external APIs
RUN apk --no-cache add ca-certificates tzdata

# Set the working directory
WORKDIR /app

# Copy all source code
COPY . .

# Debug: Check module status
RUN go env
RUN go list -m
RUN go mod tidy

# Build the Agent binary
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -v -ldflags="-w -s" -o /go/bin/agent ./cmd/agent-cli

# ==========================================
# Phase 2: Runtime Stage
# ==========================================
FROM alpine:3.21

# Re-install CA certs in the final image
RUN apk --no-cache add ca-certificates tzdata

WORKDIR /app

# Copy the strictly compiled binary from the builder stage
COPY --from=builder /go/bin/agent .

# Copy the default configuration file if it exists
COPY config.yaml .

# Provide execution permissions to the binary
RUN chmod +x ./agent

# Run the agent in interactive mode by default
ENTRYPOINT ["./agent"]
