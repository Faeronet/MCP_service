FROM golang:1.23-alpine AS builder
WORKDIR /app
RUN apk add --no-cache git ca-certificates

COPY go.mod go.sum* ./
COPY pkg/ ./pkg/
COPY bot-service/ ./bot-service/
RUN go mod tidy && go build -o /bot-service ./bot-service

FROM alpine:3.19
RUN apk add --no-cache ca-certificates tzdata tzdata tzdata
ENV TZ=Europe/Moscow
COPY --from=builder /bot-service /bot-service
EXPOSE 8081
ENTRYPOINT ["/bot-service"]
