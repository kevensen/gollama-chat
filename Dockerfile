# Build stage
FROM golang:1.24.4-alpine AS builder

# Install build dependencies including C compiler and libraries
RUN apk add --no-cache \
  git \
  ca-certificates \
  tzdata \
  gcc \
  musl-dev \
  pkgconfig

# Set working directory
WORKDIR /app

# Copy go mod and sum files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application with CGO enabled
RUN CGO_ENABLED=1 GOOS=linux go build -a -ldflags '-linkmode external -extldflags "-static"' -o gollama-chat cmd/main.go

# Execution stage
FROM alpine:latest

# Install runtime dependencies including Go for GoTTY
RUN apk --no-cache add \
  ca-certificates \
  tzdata \
  musl \
  go \
  git \
  wget

# Install GoTTY as root so it's accessible system-wide
RUN go install github.com/sorenisanerd/gotty@latest && \
  mv /root/go/bin/gotty /usr/local/bin/gotty

# Set up locale and terminal environment
ENV LANG=C.UTF-8 \
  LC_ALL=C.UTF-8 \
  TERM=xterm-256color

# Create non-root user
RUN addgroup -g 1001 -S appgroup && \
  adduser -u 1001 -S appuser -G appgroup

# Create config directory for volume mount
RUN mkdir -p /home/appuser/.config/gollama && \
  chown -R appuser:appgroup /home/appuser/.config

# Set working directory
WORKDIR /app

# Copy binary from builder stage
COPY --from=builder /app/gollama-chat .

# Change ownership to non-root user
RUN chown appuser:appgroup /app/gollama-chat

# Switch to non-root user
USER appuser

# Volume for configuration persistence
VOLUME ["/home/appuser/.config/gollama"]

# Expose port for GoTTY web interface
EXPOSE 8080

# Health check - check if GoTTY is responding
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD wget --no-verbose --tries=1 --spider http://localhost:8080/ || exit 1

# Run GoTTY with custom title and the TUI application
CMD ["gotty", "--permit-write", "--port", "8080", "--title-format", "gollama-chat", "./gollama-chat"]