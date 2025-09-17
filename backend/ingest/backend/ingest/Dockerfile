# Multi-stage build for ingest service
FROM golang:1.25-alpine AS builder

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
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o ingest ./cmd/ingest

# Final stage - distroless static image
FROM gcr.io/distroless/static-debian11

# Copy CA certificates
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

# Copy timezone data
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo

# Copy the binary
COPY --from=builder /app/ingest /ingest

# Copy schema files
COPY --from=builder /app/schemas /schemas

# Expose ports
EXPOSE 50051 9090

# Run the application
CMD ["/ingest"]
