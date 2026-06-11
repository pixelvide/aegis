# ── Stage 1: Build UI ─────────────────────────────────────────────────────────
FROM node:24-alpine AS ui-builder
RUN corepack enable && corepack prepare pnpm@10 --activate
WORKDIR /app/ui
COPY ui/package.json ui/pnpm-lock.yaml ui/.npmrc ./
RUN pnpm install --frozen-lockfile
COPY ui/ ./
RUN pnpm run build

# ── Stage 2: Build Go Server ─────────────────────────────────────────────────
FROM golang:1.25-alpine AS go-builder
RUN apk add --no-cache git
WORKDIR /app/server

# Dependencies first (cache layer)
COPY server/go.mod server/go.sum ./
RUN go mod download

# Copy source + embedded UI
COPY server/ ./
COPY --from=ui-builder /app/ui/dist ./cmd/aegis-server/ui/

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /aegis-server ./cmd/aegis-server/

# ── Stage 3: Runtime ─────────────────────────────────────────────────────────
FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata curl
RUN adduser -D -H aegis

COPY --from=go-builder /aegis-server /usr/local/bin/aegis-server
RUN chmod +x /usr/local/bin/aegis-server

USER aegis
EXPOSE 8080

HEALTHCHECK --interval=10s --timeout=3s --start-period=5s --retries=3 \
  CMD curl -f http://localhost:8080/api/v1/auth/me || exit 0

ENTRYPOINT ["aegis-server"]
