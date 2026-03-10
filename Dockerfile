# ── Stage 1: build ───────────────────────────────────────────────────────────
FROM golang:1.24-bookworm AS builder

WORKDIR /app

# Download dependencies first so they are cached when only source changes.
COPY go.mod go.sum ./
RUN go mod download

COPY . .

# go-sqlite3 requires CGO; strip debug symbols to reduce binary size.
RUN CGO_ENABLED=1 GOOS=linux \
    go build -ldflags="-s -w" -o postdoc .

# ── Stage 2: runtime ──────────────────────────────────────────────────────────
FROM debian:bookworm-slim

# ca-certificates is needed if the app ever makes outbound HTTPS requests.
# curl is used by the docker-compose healthcheck.
RUN apt-get update && \
    apt-get install -y --no-install-recommends ca-certificates curl && \
    rm -rf /var/lib/apt/lists/*

# Create an unprivileged user to run the process.
RUN useradd -u 1001 -r -s /sbin/nologin postdoc

WORKDIR /app

COPY --from=builder /app/postdoc      ./postdoc
COPY --from=builder /app/templates    ./templates

# /data holds the SQLite database file and uploaded PDFs.
RUN mkdir -p /data/uploads && chown -R postdoc:postdoc /app /data

USER postdoc

VOLUME ["/data"]

EXPOSE 8080

CMD ["/app/postdoc", \
     "-addr",    ":8080", \
     "-db",      "/data/postdoc.db", \
     "-uploads", "/data/uploads"]
