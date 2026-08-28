# Build stage: compile Go binary
FROM golang:1.26-bookworm AS builder

WORKDIR /build

# Copy dependency files first for better caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the binary (static linking for minimal runtime image)
RUN CGO_ENABLED=0 GOOS=linux go build -o chaos-server ./cmd/server

# Download kubectl for runtime (needed for kubectl exec commands)
RUN curl -LO "https://dl.k8s.io/release/$(curl -L -s https://dl.k8s.io/release/stable.txt)/bin/linux/amd64/kubectl" && \
    chmod +x kubectl

# Runtime stage: minimal image with kubectl
FROM debian:bookworm-slim

# Install ca-certificates for HTTPS calls to podinfo
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates && \
    rm -rf /var/lib/apt/lists/*

WORKDIR /app

# Copy kubectl from builder
COPY --from=builder /build/kubectl /usr/local/bin/kubectl

# Copy the binary from builder
COPY --from=builder /build/chaos-server /app/chaos-server

# Copy web assets (server uses relative paths: web/static, web/index.html)
COPY web/ /app/web/

# Create non-root user for security
RUN useradd -m -u 1000 chaos && \
    chown -R chaos:chaos /app
USER chaos

EXPOSE 8080

# Run the binary
CMD ["/app/chaos-server"]
