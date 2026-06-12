# Aegis Security Agent — Docker Runtime
#
# Pre-installed tools: semgrep, composer, npm, php, python3, go, govulncheck
# Usage:
#   make docker     (from agent/ directory — builds binaries + image)
#   docker run --rm -v /path/to/project:/workspace aegis-security:latest sharingan
#
# The workspace is mounted at /workspace. Findings are written to /workspace/.aegis/

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

# ── LocalHarness + Aegis binaries ─────────────────────────────────
# These are pre-compiled binaries built by `make docker` before the image build.
# localharness is the agent runtime engine; aegis is the security-focused wrapper.
COPY bin/localharness /usr/local/bin/localharness
COPY bin/aegis /usr/local/bin/aegis
RUN chmod +x /usr/local/bin/localharness /usr/local/bin/aegis

ENV LOCALHARNESS_BIN=/usr/local/bin/localharness

# ── Workspace mount ───────────────────────────────────────────────
WORKDIR /workspace

ENTRYPOINT ["aegis"]
CMD ["--help"]
