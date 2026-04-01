FROM golang:1.23-alpine AS builder
WORKDIR /app
RUN apk add --no-cache git ca-certificates
COPY pkg/ ./pkg/
COPY log-indexer/go.mod log-indexer/go.sum ./log-indexer/
COPY log-indexer/ ./log-indexer/
WORKDIR /app/log-indexer
RUN go mod download && CGO_ENABLED=0 go build -o /log-indexer .

FROM alpine:3.19
RUN apk add --no-cache ca-certificates
COPY --from=builder /log-indexer /log-indexer
ENTRYPOINT ["/log-indexer"]
