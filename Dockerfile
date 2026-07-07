# ─── Build Stage ──────────────────────────────────────────────────────────────
FROM golang:1.22-bookworm AS builder

WORKDIR /app

RUN apt-get update && apt-get install -y gcc libsqlite3-dev && rm -rf /var/lib/apt/lists/*

COPY go.mod go.sum ./
COPY vendor/ ./vendor/
COPY cmd/ ./cmd/
COPY internal/ ./internal/
COPY configs/ ./configs/

RUN CGO_ENABLED=0 GOFLAGS="-mod=vendor" \
    go build -ldflags="-s -w -extldflags='-static'" \
    -o nexlog-server ./cmd/server/

# ─── Runtime Stage ────────────────────────────────────────────────────────────
FROM debian:bookworm-slim

RUN apt-get update && \
    apt-get install -y --no-install-recommends ca-certificates wget && \
    rm -rf /var/lib/apt/lists/* && \
    groupadd -r app && useradd -r -g app -s /bin/false app

WORKDIR /app

COPY --from=builder /app/nexlog-server .
COPY public/ ./public/
COPY migrations/ ./migrations/

RUN mkdir -p /app/public/uploads && chown -R app:app /app

VOLUME ["/app/public/uploads"]

ENV PORT=3000
ENV PUBLIC_DIR=/app/public
ENV MIGRATIONS_DIR=/app/migrations
ENV APP_ENV=production

EXPOSE 3000

HEALTHCHECK --interval=30s --timeout=5s --start-period=15s --retries=3 \
    CMD wget -qO- http://localhost:3000/health || exit 1

USER app

CMD ["./nexlog-server"]
