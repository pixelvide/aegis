# Aegis Security Agent — Multi-Stage Docker Build
#
# Pre-installed tools: semgrep, composer, npm, php, python3, go, govulncheck
#
# Build context layout (constructed by Makefile or CI):
#   <context>/
#   ├── Dockerfile
#   ├── localharness/   ← localharness source (cloned or copied)
#   └── agent/          ← aegis agent source
#
# Usage:
#   cd agent && make docker
#   docker run --rm -v /path/to/project:/workspace aegis-agent:latest sharingan

# ── Stage 1: Build binaries ──────────────────────────────────────
FROM golang:1.25-alpine AS builder

RUN apk add --no-cache git

WORKDIR /build

# Copy localharness source
COPY localharness/ ./localharness/

# Copy agent source
COPY agent/ ./agent/

# Fix the go.mod replace directive to match Docker build layout
RUN cd agent && go mod edit -replace github.com/pixelvide/localharness=../localharness

# Download dependencies
RUN cd localharness && go mod download
RUN cd agent && go mod download

# Build localharness runtime
ARG VERSION=dev
RUN cd localharness && CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-s -w -X github.com/pixelvide/localharness/internal/config.HarnessVersion=${VERSION}" \
    -o /out/localharness ./cmd/localharness

# Build aegis agent
RUN cd agent && CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-s -w" \
    -o /out/aegis .

# ── Stage 2: Runtime ─────────────────────────────────────────────
FROM ubuntu:24.04

LABEL maintainer="pixelvide"
LABEL description="Aegis AI security research agent with pre-installed security tools"

ENV DEBIAN_FRONTEND=noninteractive
ENV PATH="/root/.local/bin:/usr/local/go/bin:/root/go/bin:${PATH}"

# ── System packages ──────────────────────────────────────────────
RUN apt-get update && apt-get install -y --no-install-recommends \
    # Core utilities
    curl wget git jq ripgrep unzip ca-certificates gnupg \
    # Network analysis
    nmap dnsutils \
    # Python (for semgrep + pip-audit)
    python3 python3-pip python3-venv pipx \
    # PHP (for composer audit + artisan)
    php-cli php-mbstring php-xml php-curl php-zip \
    # Build essentials (for native extensions)
    build-essential \
    && rm -rf /var/lib/apt/lists/*

# ── Node.js 22 (for npm audit) ───────────────────────────────────
RUN curl -fsSL https://deb.nodesource.com/setup_22.x | bash - \
    && apt-get install -y --no-install-recommends nodejs \
    && rm -rf /var/lib/apt/lists/*

# ── Go 1.24 (for govulncheck) ────────────────────────────────────
RUN wget -q https://go.dev/dl/go1.24.3.linux-amd64.tar.gz \
    && tar -C /usr/local -xzf go1.24.3.linux-amd64.tar.gz \
    && rm go1.24.3.linux-amd64.tar.gz

# ── Semgrep (SAST scanner — the #1 tool) ─────────────────────────
RUN pipx install semgrep

# ── Composer (PHP package manager + audit) ────────────────────────
RUN curl -sS https://getcomposer.org/installer | php -- --install-dir=/usr/local/bin --filename=composer

# ── Go security tools ─────────────────────────────────────────────
RUN go install golang.org/x/vuln/cmd/govulncheck@latest

# ── Python security tools ─────────────────────────────────────────
RUN pipx install pip-audit

# ── Copy pre-built binaries from builder ─────────────────────────
COPY --from=builder /out/localharness /usr/local/bin/localharness
COPY --from=builder /out/aegis /usr/local/bin/aegis

ENV LOCALHARNESS_BIN=/usr/local/bin/localharness

# ── Workspace mount ───────────────────────────────────────────────
WORKDIR /workspace

ENTRYPOINT ["aegis"]
CMD ["--help"]
