# Multi-stage Dockerfile for AegisFlux BPF Registry Service
# Stage 1: Build stage with golang:1.21-alpine
FROM golang:1.21-alpine AS builder

# Install git and ca-certificates for dependencies
RUN apk add --no-cache git ca-certificates tzdata

# Set working directory
WORKDIR /app

# Copy go mod files first for better caching
COPY go.mod ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o bpf-registry ./cmd/bpf-registry

# Create data directory in builder stage
RUN mkdir -p /data/artifacts

# Stage 2: Runtime stage with distroless
FROM gcr.io/distroless/static-debian12

# Copy the binary from builder stage
COPY --from=builder /app/bpf-registry /bpf-registry

# Copy timezone data
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo

# Copy data directory from builder stage
COPY --from=builder /data/artifacts /data/artifacts

# Expose port
EXPOSE 8090

# Set entrypoint
ENTRYPOINT ["/bpf-registry"]
