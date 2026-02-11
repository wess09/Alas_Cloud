# Build Stage
FROM golang:alpine AS builder

WORKDIR /app

# Install build dependencies
RUN apk add --no-cache gcc musl-dev

# Copy go.mod and go.sum files
COPY go.mod go.sum ./

# Copy source code
COPY . .

# Tidy and download dependencies
RUN go mod tidy && go mod download

# Build the application
# CGO_ENABLED=1 is required for go-sqlite3
RUN CGO_ENABLED=1 GOOS=linux go build -o alas-api-go ./cmd/server/main.go

# Final Stage
FROM alpine:latest

WORKDIR /app

# Install runtime dependencies (e.g., certificates, tzdata, curl)
RUN apk --no-cache add ca-certificates tzdata curl

# Copy the binary from the builder stage
COPY --from=builder /app/alas-api-go .
# Copy frontend static files
COPY --from=builder /app/frontend ./frontend

# Create necessary directories
RUN mkdir -p /app/data /app/bug_logs

# Set environment variables
ENV GIN_MODE=release \
    DATA_DIR=/app/data

# Expose port
EXPOSE 8000

# Run the binary
CMD ["./alas-api-go"]
