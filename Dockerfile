# Multi-stage build for Go application
FROM golang:1.25-alpine AS builder

# Install git and ca-certificates
RUN apk add --no-cache git ca-certificates

# Set working directory
WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o bot ./cmd/bot

# Final stage - Debian slim
FROM debian:bullseye-slim

# Install ca-certificates for HTTPS requests
RUN apt-get update && apt-get install -y \
    ca-certificates \
    && rm -rf /var/lib/apt/lists/*

# Create non-root user
RUN useradd -m -u 1000 botuser

# Set working directory
WORKDIR /app

# Copy binary from builder stage
COPY --from=builder /app/bot .

# Create downloads directory and set permissions
RUN mkdir -p /app/downloads && chown -R botuser:botuser /app

# Switch to non-root user
USER botuser

# Expose volume for downloads
VOLUME ["/app/downloads"]

# Set environment variables
ENV DOWNLOAD_FOLDER=/app/downloads

# Run the application
CMD ["./bot"]