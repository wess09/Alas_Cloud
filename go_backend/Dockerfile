# Build Stage
FROM golang:1.21-alpine AS builder

WORKDIR /app

# Install build dependencies
RUN apk add --no-cache gcc musl-dev

# Copy go.mod and go.sum files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application
# CGO_ENABLED=1 is required for go-sqlite3
RUN CGO_ENABLED=1 GOOS=linux go build -o alas-api-go ./cmd/server/main.go

# Final Stage
FROM alpine:latest

WORKDIR /app

# Install runtime dependencies (e.g., certificates, tzdata)
RUN apk --no-cache add ca-certificates tzdata

# Copy the binary from the builder stage
COPY --from=builder /app/alas-api-go .

# Create necessary directories
RUN mkdir -p /app/data /app/bug_logs

# Set environment variables
ENV GIN_MODE=release \
    DATA_DIR=/app/data

# Expose port
EXPOSE 8000

# Run the binary
CMD ["./alas-api-go"]
