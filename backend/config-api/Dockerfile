# Multi-stage Dockerfile for AegisFlux Config API Service
# Stage 1: Build stage with golang:1.23-alpine
FROM golang:1.23-alpine AS builder

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
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o config-api ./cmd/config-api

# Stage 2: Runtime stage with distroless
FROM gcr.io/distroless/static-debian12

# Copy the binary from builder stage
COPY --from=builder /app/config-api /config-api

# Copy timezone data
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo

# Expose port
EXPOSE 8085

# Set the binary as the entrypoint
ENTRYPOINT ["/config-api"]
