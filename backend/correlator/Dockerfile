# Multi-stage Dockerfile for AegisFlux Correlator Service

# Build stage
FROM golang:1.21-alpine AS builder

# Install git and ca-certificates for dependencies
RUN apk add --no-cache git ca-certificates tzdata

# Set working directory
WORKDIR /app

# Copy go mod files first for better caching
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o correlator ./cmd/correlator

# Runtime stage
FROM gcr.io/distroless/static-debian12

# Copy CA certificates from builder
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

# Copy timezone data from builder
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo

# Copy the binary from builder
COPY --from=builder /app/correlator /correlator

# Copy default rules
COPY --from=builder /app/rules.d /rules.d

# Create non-root user
USER nonroot:nonroot

# Expose port 8082
EXPOSE 8082

# Set environment variables
ENV CORR_HTTP_ADDR=:8082
ENV CORR_RULES_DIR=/rules.d
ENV CORR_NATS_URL=nats://localhost:4222
ENV CORR_MAX_FINDINGS=10000
ENV CORR_DEDUPE_CAP=100000
ENV CORR_HOT_RELOAD=false
ENV CORR_DEBOUNCE_MS=1000
ENV CORR_LABEL_TTL_SEC=300

# Health check - using external health check via orchestrator
# HEALTHCHECK is handled by the orchestrator/container runtime

# Run the application
ENTRYPOINT ["/correlator"]
