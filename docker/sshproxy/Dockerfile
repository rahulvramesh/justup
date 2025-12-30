# Justup SSH Proxy
# Multi-stage build for minimal image size

# Build stage
FROM golang:1.22-alpine AS builder

RUN apk add --no-cache gcc musl-dev sqlite-dev

WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the SSH proxy binary
RUN CGO_ENABLED=1 GOOS=linux go build -a -ldflags '-linkmode external -extldflags "-static"' -o /sshproxy ./cmd/sshproxy

# Runtime stage
FROM alpine:3.19

RUN apk add --no-cache ca-certificates sqlite

# Create non-root user
RUN addgroup -S justup && adduser -S justup -G justup

# Create directories
RUN mkdir -p /etc/justup /var/lib/justup && \
    chown -R justup:justup /etc/justup /var/lib/justup

# Copy binary
COPY --from=builder /sshproxy /usr/local/bin/sshproxy

# Use non-root user
USER justup

# Expose SSH port
EXPOSE 2222

# Health check
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD nc -z localhost 2222 || exit 1

ENTRYPOINT ["/usr/local/bin/sshproxy"]
CMD ["--addr", ":2222", "--host-key", "/etc/justup/ssh_host_ed25519_key", "--db", "/var/lib/justup/justup.db"]
