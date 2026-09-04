# Build stage
FROM golang:1.26-alpine3.24 AS builder

WORKDIR /app

# Cache dependencies first
COPY go.mod go.sum* ./
RUN go mod download

# Copy source and build
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -ldflags="-s -w -X main.version=v0.1.0" -o /crawler ./cmd/crawler

# Runtime stage
FROM alpine:latest

RUN addgroup -S crawler && adduser -S -G crawler crawler

WORKDIR /app

# Copy binary from builder
COPY --from=builder /crawler /usr/local/bin/crawler

# Create directories
RUN mkdir -p /app/registry /app/data /app/config && \
    chown -R crawler:crawler /app/registry /app/data /app/config

USER crawler

EXPOSE 8080

ENTRYPOINT ["crawler"]
CMD ["version"]
