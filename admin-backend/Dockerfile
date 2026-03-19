FROM golang:1.23-alpine AS builder
WORKDIR /app
RUN apk add --no-cache git ca-certificates

COPY go.mod go.sum* ./
COPY pkg/ ./pkg/
RUN mkdir -p ./admin-backend ./admin-backend/modules
COPY admin-backend/main.go admin-backend/main_build.go ./admin-backend/
COPY admin-backend/modules/ ./admin-backend/modules/
RUN go mod tidy && go build -tags docker -o /admin-backend ./admin-backend

# Финальный образ без nvidia/cuda. Метрики GPU в мониторе — через nvidia-smi на хосте (если смонтирован в PATH) или пусто.
FROM debian:bookworm-slim
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates wget && rm -rf /var/lib/apt/lists/*
COPY --from=builder /admin-backend /admin-backend
EXPOSE 8080
ENTRYPOINT ["/admin-backend"]
