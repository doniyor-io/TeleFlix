FROM golang:1.25-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o bot_binary cmd/bot/main.go

FROM alpine:latest

WORKDIR /app

COPY --from=builder /app/bot_binary .
COPY --from=builder /app/locales ./locales
COPY --from=builder /app/.env .

CMD ["./bot_binary"]