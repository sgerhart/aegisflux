# Multi-stage Dockerfile for AegisFlux Decision Service
# Stage 1: Build stage with golang:1.23-alpine
FROM golang:1.23-alpine AS builder

# Install git and ca-certificates for dependencies
RUN apk add --no-cache git ca-certificates tzdata

# Set working directory
WORKDIR /app

# Copy go mod files first for better caching
COPY backend/decision/go.mod backend/decision/go.sum ./

# Copy the common module that decision depends on
COPY backend/common/ ../common/

# Download dependencies
RUN go mod download

# Copy source code
COPY backend/decision/ .

# Copy LLM system prompts and models configuration
COPY llm/system-prompts/ ./llm/system-prompts/
COPY llm/models.yaml ./llm/models.yaml

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o decision ./cmd/decision

# Stage 2: Runtime stage with distroless
FROM gcr.io/distroless/static-debian12

# Copy the binary from builder stage
COPY --from=builder /app/decision /decision

# Copy LLM system prompts and models configuration
COPY --from=builder /app/llm/system-prompts/ /llm/system-prompts/
COPY --from=builder /app/llm/models.yaml /llm/models.yaml

# Copy timezone data
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo

# Expose port
EXPOSE 8083

# Set the binary as the entrypoint
ENTRYPOINT ["/decision"]
