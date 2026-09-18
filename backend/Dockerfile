FROM golang:1.26-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN go build -o main ./cmd/main.go

FROM alpine:latest
WORKDIR /app
COPY --from=builder /app/main .

EXPOSE 3000
CMD ["./main"]
