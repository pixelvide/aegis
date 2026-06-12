.PHONY: build docker docker-run clean

# ── Configuration ─────────────────────────────────────────────────
LOCALHARNESS_REPO := ../../local-harness
IMAGE := aegis-agent:latest
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "0.0.0-dev")

# ── Build Aegis binary (native) ──────────────────────────────────
build:
	@echo "==> Building aegis agent..."
	go build -o bin/aegis .
	@echo "==> Built bin/aegis"

# ── Build Docker image ───────────────────────────────────────────
# Cross-compiles both localharness and aegis for linux/amd64,
# then builds the Docker image with pre-installed security tools.
docker:
	@echo "==> Building Aegis Docker image..."
	@mkdir -p bin

	@echo "  → Cross-compiling localharness (linux/amd64)..."
	cd $(LOCALHARNESS_REPO) && \
		GOOS=linux GOARCH=amd64 go build \
		-ldflags="-s -w -X github.com/pixelvide/localharness/internal/config.HarnessVersion=$(VERSION)" \
		-o $(CURDIR)/bin/localharness ./cmd/localharness

	@echo "  → Cross-compiling aegis (linux/amd64)..."
	GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o bin/aegis .

	@echo "  → Building Docker image $(IMAGE)..."
	docker build -t $(IMAGE) .

	@rm -rf bin
	@echo "==> Built $(IMAGE)"
	@echo "    Run: ./aegis-run sharingan --workspace=/path/to/project"

# ── Run in Docker (convenience) ──────────────────────────────────
# Usage: make docker-run WORKSPACE=/path/to/project PERSONA=sharingan
docker-run: docker
	./aegis-run --workspace=$(WORKSPACE) $(PERSONA)

# ── Clean ─────────────────────────────────────────────────────────
clean:
	rm -rf bin
