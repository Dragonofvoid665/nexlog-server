# ─── Build Stage ──────────────────────────────────────────────────────────────
FROM golang:1.22-bookworm AS builder

WORKDIR /app

# Install SQLite for CGO
RUN apt-get update && apt-get install -y gcc libsqlite3-dev && rm -rf /var/lib/apt/lists/*

COPY go.mod go.sum ./
COPY vendor/ ./vendor/
COPY cmd/ ./cmd/
COPY internal/ ./internal/

RUN CGO_ENABLED=1 GOFLAGS="-mod=vendor" go build -ldflags="-s -w" -o nexlog-server ./cmd/server/

# ─── Runtime Stage ────────────────────────────────────────────────────────────
FROM debian:bookworm-slim

RUN apt-get update && apt-get install -y libsqlite3-0 ca-certificates && rm -rf /var/lib/apt/lists/*

WORKDIR /app

COPY --from=builder /app/nexlog-server .
COPY public/ ./public/

RUN mkdir -p /app/data /app/public/uploads \
    && chmod -R 777 /app/data /app/public/uploads

ENV PORT=3000
ENV DATA_DIR=/app/data
ENV PUBLIC_DIR=/app/public

EXPOSE 3000

CMD ["./nexlog-server"]
