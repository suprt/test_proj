##############################
# Build stage
##############################
FROM golang:1.24.6-alpine AS builder

WORKDIR /app

# Speed up modules layer caching
COPY go.mod go.sum ./
RUN go mod download

# Copy only relevant source directories to improve cache hit rate
COPY api/ api/
COPY cmd/ cmd/
COPY internal/ internal/
COPY README.md .

# Build static binaries with smaller size
ENV CGO_ENABLED=0
RUN go build -trimpath -ldflags="-s -w" -o server ./cmd/server
RUN go build -trimpath -ldflags="-s -w" -o client ./cmd/client

##############################
# Server runtime stage
##############################
FROM alpine:latest AS server

# Install CA certificates for gRPC and netcat for healthcheck
RUN apk --no-cache add ca-certificates netcat-openbsd

WORKDIR /app

# Copy only server binary
COPY --from=builder /app/server .

# Create data directory
RUN mkdir -p /app/data

# Expose gRPC port
EXPOSE 50051

# Default command: run the server
CMD ["./server", "-addr", ":50051", "-data", "/app/data"]

##############################
# Client runtime stage
##############################
FROM alpine:latest AS client

# Install CA certificates for gRPC and file utility for tests
RUN apk --no-cache add ca-certificates file

WORKDIR /app

# Copy only client binary
COPY --from=builder /app/client .

# Default command: show help
CMD ["./client", "-h"]
