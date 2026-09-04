# Build stage
FROM golang:1.26-alpine3.24 AS builder

WORKDIR /src

# Copy go mod files and download deps (cache layer)
COPY go.mod go.sum ./
RUN go mod download

# Copy source
COPY . .

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w -X main.version=v1.0.0 -X main.commit=docker -X main.date=$(date -u +%Y-%m-%dT%H:%M:%SZ)" -o /crawler ./cmd/crawler

# Runtime stage
FROM alpine:latest

# Create non-root user
RUN addgroup -S -g 1000 crawler && \
    adduser -S -u 1000 -G crawler crawler

# Install ca-certificates for HTTPS
RUN apk --no-cache add ca-certificates && \
    rm -rf /var/cache/apk/*

WORKDIR /app

# Copy the binary
COPY --from=builder /crawler /usr/local/bin/crawler

# Create data directories with proper ownership
RUN mkdir -p /data/registry /data/db /app/config && \
    chown -R crawler:crawler /data /app

# Switch to non-root user
USER crawler

# Volume mounts for data persistence
VOLUME ["/data/registry", "/data/db", "/app/config"]

# Default command
ENTRYPOINT ["crawler"]
CMD ["version"]
