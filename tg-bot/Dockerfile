FROM golang:1.23-alpine AS builder
WORKDIR /app
RUN apk add --no-cache git ca-certificates

COPY go.mod go.sum* ./
COPY pkg/ ./pkg/
COPY tg-bot/ ./tg-bot/
RUN go mod tidy && go build -o /tg-bot ./tg-bot

FROM alpine:3.19
RUN apk add --no-cache ca-certificates
COPY --from=builder /tg-bot /tg-bot
EXPOSE 8081
ENTRYPOINT ["/tg-bot"]
