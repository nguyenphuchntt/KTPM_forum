# Use official golang image with specific version
FROM golang:1.24-alpine

# Install build dependencies
RUN apk add --no-cache gcc musl-dev sqlite-dev

WORKDIR /app

COPY go.mod go.sum ./

RUN go mod download

COPY . .

ENV BASE_PATH="/app/"

RUN CGO_ENABLED=1 go build -o forum ./cmd/main.go

EXPOSE 8080

CMD ["sh", "-c", "./forum --migrate && ./forum --seed && ./forum"]
