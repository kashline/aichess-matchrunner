# ----------- Build Stage ----------- #
FROM golang:1.24-alpine AS builder

# Install git (required for go get) and CA certs
RUN apk add --no-cache git ca-certificates

# Set working directory
WORKDIR /app

# Copy go mod and sum files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy the rest of the code
COPY . .

# Build the app
RUN go build -o server /app/cmd

# ----------- Run Stage ----------- #
FROM alpine:latest

# Install CA certificates (needed for TLS)
RUN apk --no-cache add ca-certificates

# Set working directory
WORKDIR /root/

# Copy the binary from build stage
COPY --from=builder /app/server .

# Expose port (change if your app uses another port)
EXPOSE 8080

# Run the binary
CMD ["./server"]
