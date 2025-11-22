# Use official golang image with specific version
FROM golang:1.22.3-alpine

WORKDIR /app

COPY go.mod go.sum ./

RUN go mod download

COPY . .

ENV BASE_PATH="/app/"

RUN go build -o forum ./cmd/main.go

EXPOSE 8080

CMD ["./forum"]