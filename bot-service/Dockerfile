# Use ghcr.io if Docker Hub is unreachable: --build-arg REGISTRY=ghcr.io/docker-library/
ARG REGISTRY=
FROM ${REGISTRY}golang:1.22-alpine AS builder
WORKDIR /app
RUN apk add --no-cache git ca-certificates

COPY go.mod go.sum* ./
COPY pkg/ ./pkg/
COPY bot-service/ ./bot-service/
RUN go mod tidy && go build -o /bot-service ./bot-service

FROM ${REGISTRY}alpine:3.19
RUN apk add --no-cache ca-certificates
COPY --from=builder /bot-service /bot-service
EXPOSE 8081
ENTRYPOINT ["/bot-service"]
