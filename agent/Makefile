.PHONY: build docker docker-run clean

# ── Configuration ─────────────────────────────────────────────────
LOCALHARNESS_REPO := ../../local-harness
IMAGE := aegis-agent:latest
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "0.0.0-dev")

# ── Build Aegis binary (native, for local dev) ───────────────────
build:
	@echo "==> Building aegis agent..."
	@mkdir -p bin
	go build -o bin/aegis .
	@echo "==> Built bin/aegis"

# ── Build Docker image ───────────────────────────────────────────
# Constructs a build context with both localharness and agent source,
# then runs a multi-stage Docker build (Go compilation happens inside Docker).
docker:
	@echo "==> Building Aegis Docker image..."
	@mkdir -p .build-context

	@echo "  → Copying localharness source..."
	@rsync -a --exclude='.git' --exclude='bin' --exclude='dist' \
		$(LOCALHARNESS_REPO)/ .build-context/localharness/

	@echo "  → Copying agent source..."
	@rsync -a --exclude='.git' --exclude='bin' --exclude='.build-context' \
		. .build-context/agent/

	@cp Dockerfile .build-context/Dockerfile

	@echo "  → Building Docker image $(IMAGE)..."
	docker build --build-arg VERSION=$(VERSION) -t $(IMAGE) .build-context

	@rm -rf .build-context
	@echo "==> Built $(IMAGE)"
	@echo "    Run: ./aegis-run sharingan --workspace=/path/to/project"

# ── Run in Docker (convenience) ──────────────────────────────────
# Usage: make docker-run WORKSPACE=/path/to/project PERSONA=sharingan
docker-run: docker
	./aegis-run --workspace=$(WORKSPACE) $(PERSONA)

# ── Clean ─────────────────────────────────────────────────────────
clean:
	rm -rf bin .build-context
