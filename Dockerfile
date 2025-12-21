# Build stage
FROM golang:1.21-alpine AS builder

WORKDIR /app

# Install build dependencies
RUN apk add --no-cache git

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o mypatroni ./cmd/mypatroni

# Final stage
FROM alpine:3.19

RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app

# Copy binary from builder
COPY --from=builder /app/mypatroni /usr/local/bin/mypatroni

# Create config directory
RUN mkdir -p /etc/mypatroni /var/log/mypatroni

# Copy example config
COPY configs/example.yaml /etc/mypatroni/config.yaml.example

# Create non-root user
RUN adduser -D -u 1000 mypatroni
USER mypatroni

EXPOSE 8080

ENTRYPOINT ["mypatroni"]
CMD ["--config", "/etc/mypatroni/config.yaml"]
