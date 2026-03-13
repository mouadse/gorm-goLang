# Build stage
FROM golang:1.25-alpine AS builder

WORKDIR /app

# Install dependencies for building
RUN apk add --no-cache git ca-certificates tzdata

# Copy go mod files first for better caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o /fitness-tracker .

# Runtime stage
FROM alpine:3.19

WORKDIR /app

# Install runtime dependencies, including wget for container healthchecks.
RUN apk --no-cache add ca-certificates tzdata wget

# Copy binary from builder
COPY --from=builder /fitness-tracker /app/fitness-tracker

# Create non-root user
RUN adduser -D -g '' appuser
USER appuser

EXPOSE 8080

ENTRYPOINT ["/app/fitness-tracker"]
