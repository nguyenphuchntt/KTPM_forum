# Use official golang image with specific version
FROM golang:1.22.3-alpine

# Install required dependencies
RUN apk add --no-cache gcc musl-dev

# Set working directory
WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./

# Copy source code
COPY . .

RUN go mod tidy

# Download dependencies
RUN go mod download

# Set environment variable for template base path
ENV BASE_PATH="/app/"

# Build the application
RUN go build -o forum ./cmd/main.go

# Expose port 8080
EXPOSE 8080

# Command to run the application
CMD ["./forum"]