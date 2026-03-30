FROM golang:1.23-alpine AS builder
WORKDIR /app
RUN apk add --no-cache git ca-certificates

COPY go.mod go.sum* ./
COPY pkg/ ./pkg/
COPY scheduler/ ./scheduler/
RUN go mod tidy && CGO_ENABLED=0 go build -o /scheduler ./scheduler

FROM alpine:3.19
RUN apk add --no-cache ca-certificates tzdata wget
ENV TZ=Europe/Moscow
COPY --from=builder /scheduler /scheduler
EXPOSE 8090
ENTRYPOINT ["/scheduler"]
