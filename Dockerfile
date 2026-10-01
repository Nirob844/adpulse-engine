# Multi-stage Dockerfile for Mini Ad Server
# Build stage
FROM golang:alpine AS builder

WORKDIR /app

# Install security certificates and git
RUN apk add --no-cache git ca-certificates tzdata

# Download dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Compile high-performance static binary with optimizations
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-w -s -extldflags '-static'" \
    -o /app/bin/server ./cmd/server

# Final minimal runtime stage
FROM alpine:3.20

WORKDIR /app

RUN apk add --no-cache ca-certificates tzdata curl

# Copy compiled binary from builder
COPY --from=builder /app/bin/server /app/server

# Copy static frontend assets & migrations
COPY --from=builder /app/web /app/web
COPY --from=builder /app/migrations /app/migrations

# Run with non-privileged user for security
RUN adduser -D -u 10001 appuser && \
    chown -R appuser:appuser /app
USER appuser

EXPOSE 8000

ENV SERVER_PORT=8000 \
    APP_ENV=production

ENTRYPOINT ["/app/server"]
