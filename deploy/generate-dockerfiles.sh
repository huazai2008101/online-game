#!/bin/bash
# Generate Dockerfiles for all services

SERVICES=(
    "game-service:8002"
    "payment-service:8003"
    "player-service:8004"
    "activity-service:8005"
    "guild-service:8006"
    "item-service:8007"
    "notification-service:8008"
    "organization-service:8009"
    "permission-service:8010"
    "id-service:8011"
    "file-service:8012"
)

DOCKER_DIR="deploy/docker"
mkdir -p "$DOCKER_DIR"

for SERVICE_INFO in "${SERVICES[@]}"; do
    IFS=':' read -r SERVICE_NAME SERVICE_PORT <<< "$SERVICE_INFO"

    cat > "$DOCKER_DIR/Dockerfile.$SERVICE_NAME" <<EOF
# Build stage
FROM golang:1.23-alpine AS builder

WORKDIR /app

# Install build dependencies
RUN apk add --no-cache git make

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the service
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o $SERVICE_NAME ./cmd/$SERVICE_NAME

# Runtime stage
FROM alpine:latest

WORKDIR /app

# Install ca-certificates for HTTPS
RUN apk --no-cache add ca-certificates curl tzdata

# Copy binary from builder
COPY --from=builder /app/$SERVICE_NAME .

# Create non-root user
RUN addgroup -g 1000 appuser && \\
    adduser -D -u 1000 -G appuser appuser && \\
    chown -R appuser:appuser /app

USER appuser

# Health check
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \\
    CMD curl -f http://localhost:$SERVICE_PORT/health || exit 1

EXPOSE $SERVICE_PORT

CMD ["./$SERVICE_NAME"]
EOF

    echo "Created $DOCKER_DIR/Dockerfile.$SERVICE_NAME"
done

echo "All Dockerfiles generated successfully!"
