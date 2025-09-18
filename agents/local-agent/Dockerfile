# Build stage
FROM golang:1.21-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git ca-certificates tzdata

# Set working directory
WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o local-agent .

# Runtime stage
FROM alpine:latest

# Install runtime dependencies
RUN apk --no-cache add ca-certificates tzdata

# Create non-root user
RUN addgroup -g 1001 -S agent && \
    adduser -u 1001 -S agent -G agent

# Set working directory
WORKDIR /app

# Copy binary from builder stage
COPY --from=builder /app/local-agent .

# Copy timezone data
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo

# Create cache directory
RUN mkdir -p /tmp/aegisflux-agent && \
    chown -R agent:agent /tmp/aegisflux-agent

# Switch to non-root user
USER agent

# Expose port (if needed for health checks)
EXPOSE 8080

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD pgrep local-agent || exit 1

# Default environment variables
ENV AGENT_HOST_ID=localhost
ENV AGENT_REGISTRY_URL=http://localhost:8084
ENV AGENT_POLL_INTERVAL_SEC=30
ENV AGENT_NATS_URL=nats://localhost:4222
ENV AGENT_VAULT_URL=http://localhost:8200
ENV AGENT_VAULT_TOKEN=dev-token
ENV AGENT_CACHE_DIR=/tmp/aegisflux-agent
ENV AGENT_MAX_PROGRAMS=10
ENV AGENT_DEFAULT_TTL_SEC=3600
ENV AGENT_LOG_LEVEL=info
ENV AGENT_TELEMETRY_SUBJECT=agent.telemetry

# Run the agent
CMD ["./local-agent"]
