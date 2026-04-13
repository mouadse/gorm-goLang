# syntax=docker/dockerfile:1.7

# Build stage
FROM golang:1.25-alpine AS builder

WORKDIR /app

# Copy go mod files first for better caching
COPY --link go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

# Copy source code
COPY --link . .

# Build both binaries in one cached layer so shared packages are compiled once.
RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg/mod \
    mkdir -p /out && \
    CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-w -s" -o /out/fitness-tracker . && \
    CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-w -s" -o /out/fitness-tracker-seed ./seed

# Runtime stage
FROM alpine:3.19

WORKDIR /app

# Install runtime dependencies. Alpine already includes wget for healthchecks.
RUN apk --no-cache add ca-certificates tzdata

# Create non-root user and group so --chown resolves reliably.
RUN addgroup -S appuser && adduser -S -G appuser appuser

# Copy binaries and bundled USDA dataset
COPY --link --chown=appuser:appuser --from=builder /out/fitness-tracker /app/fitness-tracker
COPY --link --chown=appuser:appuser --from=builder /out/fitness-tracker-seed /app/fitness-tracker-seed
COPY --link --chown=appuser:appuser FoodData_Central_foundation_food_json_2025-12-18.json /app/FoodData_Central_foundation_food_json_2025-12-18.json

USER appuser

EXPOSE 8080

ENTRYPOINT ["/app/fitness-tracker"]
