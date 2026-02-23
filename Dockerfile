# ==========================================
# Phase 1: Go Builder
# ==========================================
FROM golang:1.24-alpine AS builder

RUN apk --no-cache add ca-certificates tzdata git

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN go mod tidy

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -v -ldflags="-w -s" -o /go/bin/agent ./cmd/agent-cli

# ==========================================
# Phase 2: Tools Installer
# ==========================================
FROM debian:bookworm-slim AS tools-installer

RUN apt-get update && apt-get install -y --no-install-recommends \
    curl wget git ca-certificates unzip tar \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /tools

# ── Trivy ─────────────────────────────────────────────────────────────────────
RUN curl -sfL https://raw.githubusercontent.com/aquasecurity/trivy/main/contrib/install.sh \
    | sh -s -- -b /usr/local/bin

# ── Grype ─────────────────────────────────────────────────────────────────────
RUN curl -sSfL https://raw.githubusercontent.com/anchore/grype/main/install.sh \
    | sh -s -- -b /usr/local/bin

# ── Syft ──────────────────────────────────────────────────────────────────────
RUN curl -sSfL https://raw.githubusercontent.com/anchore/syft/main/install.sh \
    | sh -s -- -b /usr/local/bin

# ── Gitleaks ──────────────────────────────────────────────────────────────────
RUN GITLEAKS_VERSION=$(curl -s https://api.github.com/repos/gitleaks/gitleaks/releases/latest \
    | grep '"tag_name"' | cut -d'"' -f4 | tr -d 'v') && \
    curl -sSfL "https://github.com/gitleaks/gitleaks/releases/download/v${GITLEAKS_VERSION}/gitleaks_${GITLEAKS_VERSION}_linux_x64.tar.gz" \
    | tar -xz -C /usr/local/bin gitleaks

# ── TruffleHog ────────────────────────────────────────────────────────────────
RUN curl -sSfL https://raw.githubusercontent.com/trufflesecurity/trufflehog/main/scripts/install.sh \
    | sh -s -- -b /usr/local/bin

# ── OSV-Scanner ───────────────────────────────────────────────────────────────
RUN curl -sSfL "https://github.com/google/osv-scanner/releases/latest/download/osv-scanner_linux_amd64" \
    -o /usr/local/bin/osv-scanner && chmod +x /usr/local/bin/osv-scanner

# ── TFSec ─────────────────────────────────────────────────────────────────────
RUN curl -sSfL "https://github.com/aquasecurity/tfsec/releases/latest/download/tfsec-linux-amd64" \
    -o /usr/local/bin/tfsec && chmod +x /usr/local/bin/tfsec

# ── Terrascan ─────────────────────────────────────────────────────────────────
RUN curl -sSfL "https://github.com/tenable/terrascan/releases/latest/download/terrascan_Linux_x86_64.tar.gz" \
    | tar -xz -C /usr/local/bin terrascan

# ── Gosec ─────────────────────────────────────────────────────────────────────
RUN curl -sSfL https://raw.githubusercontent.com/securego/gosec/master/install.sh \
    | sh -s -- -b /usr/local/bin

# ── Nancy ─────────────────────────────────────────────────────────────────────
RUN curl -sSfL "https://github.com/sonatype-nexus-community/nancy/releases/latest/download/nancy-linux.amd64-musl" \
    -o /usr/local/bin/nancy && chmod +x /usr/local/bin/nancy

# ── Gobuster ──────────────────────────────────────────────────────────────────
RUN GOBUSTER_VERSION=$(curl -s https://api.github.com/repos/OJ/gobuster/releases/latest \
    | grep '"tag_name"' | cut -d'"' -f4 | tr -d 'v') && \
    curl -sSfL "https://github.com/OJ/gobuster/releases/download/v${GOBUSTER_VERSION}/gobuster_Linux_x86_64.tar.gz" \
    | tar -xz -C /usr/local/bin gobuster

# ── Checkov standalone binary ─────────────────────────────────────────────────
RUN curl -sSfL "https://github.com/bridgecrewio/checkov/releases/latest/download/checkov_linux_X86_64.zip" \
    -o /tmp/checkov.zip && \
    unzip /tmp/checkov.zip -d /usr/local/bin && \
    chmod +x /usr/local/bin/checkov && \
    rm /tmp/checkov.zip

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
    ca-certificates tzdata curl wget git jq sudo \
    # DAST
    nmap nikto sqlmap \
    # Python
    python3 python3-pip \
    # Chromium + Playwright deps
    chromium \
    fonts-liberation libatk-bridge2.0-0 libatk1.0-0 \
    libcups2 libdbus-1-3 libgdk-pixbuf2.0-0 libnspr4 \
    libnss3 libx11-xcb1 libxcomposite1 libxdamage1 \
    libxrandr2 libgbm1 libxss1 libasound2 \
    libnss3-tools \
    && apt-get clean && rm -rf /var/lib/apt/lists/*

# ── nmap capabilities بدون root ───────────────────────────────────────────────
RUN setcap cap_net_raw,cap_net_admin,cap_net_bind_service+eip $(which nmap)

# ── Python Security Tools ──────────────────────────────────────────────────────
RUN pip3 install --break-system-packages \
    semgrep \
    bandit \
    detect-secrets \
    safety \
    playwright

# ── Playwright: system Chromium بدل تنزيل browser تاني ──────────────────────
ENV PLAYWRIGHT_BROWSERS_PATH=/root/.cache/ms-playwright
ENV PLAYWRIGHT_CHROMIUM_EXECUTABLE_PATH=/usr/bin/chromium
RUN playwright install chromium 2>/dev/null || true

# ── Copy prebuilt binaries ────────────────────────────────────────────────────
COPY --from=tools-installer /usr/local/bin/trivy        /usr/local/bin/
COPY --from=tools-installer /usr/local/bin/grype        /usr/local/bin/
COPY --from=tools-installer /usr/local/bin/syft         /usr/local/bin/
COPY --from=tools-installer /usr/local/bin/gitleaks     /usr/local/bin/
COPY --from=tools-installer /usr/local/bin/trufflehog   /usr/local/bin/
COPY --from=tools-installer /usr/local/bin/osv-scanner  /usr/local/bin/
COPY --from=tools-installer /usr/local/bin/tfsec        /usr/local/bin/
COPY --from=tools-installer /usr/local/bin/terrascan    /usr/local/bin/
COPY --from=tools-installer /usr/local/bin/gosec        /usr/local/bin/
COPY --from=tools-installer /usr/local/bin/nancy        /usr/local/bin/
COPY --from=tools-installer /usr/local/bin/gobuster     /usr/local/bin/
COPY --from=tools-installer /usr/local/bin/checkov      /usr/local/bin/

# ── Copy compiled Go agent ────────────────────────────────────────────────────
WORKDIR /app
COPY --from=builder /go/bin/agent .
COPY config.yaml .
RUN chmod +x ./agent && chown -R duckops:duckops /app

# ── Verify all tools at build time ────────────────────────────────────────────
RUN echo "=== Verifying tools ===" && \
    nmap        --version | head -1 && \
    nikto       -Version 2>&1 | head -1 && \
    sqlmap      --version 2>&1 | head -1 && \
    gobuster    version && \
    semgrep     --version && \
    bandit      --version && \
    gosec       --version 2>&1 | head -1 && \
    trufflehog  --version && \
    gitleaks    version && \
    trivy       --version | head -1 && \
    grype       version | head -1 && \
    syft        --version && \
    osv-scanner --version && \
    safety      --version && \
    checkov     --version && \
    tfsec       --version && \
    terrascan   version && \
    nancy       --version && \
    echo "=== All tools OK ==="

# ── Environment ───────────────────────────────────────────────────────────────
ENV LANG=en_US.UTF-8
ENV TZ=UTC

# ── Switch to non-root ────────────────────────────────────────────────────────
USER duckops
WORKDIR /workspace

ENTRYPOINT ["/app/agent"]