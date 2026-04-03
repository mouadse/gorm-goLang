# syntax=docker/dockerfile:1

# Build stage
FROM golang:1.25-alpine AS builder

WORKDIR /app

# Copy go mod files first for better caching
COPY --link go.mod go.sum ./
RUN go mod download

# Copy source code
COPY --link . .

# Build the application with persistent Go build caches for faster rebuilds.
RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg/mod \
    CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o /fitness-tracker .

# Runtime stage
FROM alpine:3.19

WORKDIR /app

# Install runtime dependencies. Alpine already includes wget for healthchecks.
RUN apk --no-cache add ca-certificates tzdata

# Copy binary from builder
COPY --link --from=builder /fitness-tracker /app/fitness-tracker

# Create non-root user
RUN adduser -D -g '' appuser
USER appuser

EXPOSE 8080

ENTRYPOINT ["/app/fitness-tracker"]
