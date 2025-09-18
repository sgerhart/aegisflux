# Go build stage
FROM golang:1.21-alpine AS go-build
RUN apk add --no-cache git build-base
WORKDIR /src

# Copy go mod files
COPY backend/common/go.mod ../common/
COPY backend/orchestrator/go.mod backend/orchestrator/go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY backend/common/ ../common/
COPY backend/orchestrator/ .

# Build the application
RUN CGO_ENABLED=0 go build -o /out/orchestrator ./cmd/orchestrator/main_simple.go

# eBPF build stage
FROM alpine:3.20 AS ebpf-build
RUN apk add --no-cache clang llvm bpftools make zstd
WORKDIR /app
# Templates will be bind-mounted at runtime
COPY ../../bpf-templates /app/bpf-templates

# Pre-build one template to prove toolchain
RUN make -C /app/bpf-templates/drop_egress_by_cgroup

# Final stage
FROM gcr.io/distroless/base-debian12
COPY --from=go-build /out/orchestrator /orchestrator
# Templates come via bind mount at runtime
EXPOSE 8084
ENTRYPOINT ["/orchestrator"]
