FROM golang:1.23-alpine AS builder
WORKDIR /app
RUN apk add --no-cache git ca-certificates
COPY pkg/ ./pkg/
COPY go.mod go.sum ./
COPY main.go ./
COPY modules/ ./modules/
RUN go mod download && CGO_ENABLED=0 go build -o /tg-bot .

FROM alpine:3.19
RUN apk add --no-cache ca-certificates tzdata wget
COPY --from=builder /tg-bot /tg-bot
EXPOSE 8081
ENTRYPOINT ["/tg-bot"]
