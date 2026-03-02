FROM golang:1.23-alpine AS builder
WORKDIR /app
RUN apk add --no-cache git ca-certificates

COPY go.mod go.sum* ./
COPY pkg/ ./pkg/
RUN mkdir -p ./admin-backend
COPY admin-backend/handler.go admin-backend/main_build.go admin-backend/metrics.go admin-backend/docker_stats.go ./
RUN mv main_build.go admin-backend/main.go && mv handler.go admin-backend/ && mv metrics.go admin-backend/ && mv docker_stats.go admin-backend/
RUN go mod tidy && go build -o /admin-backend ./admin-backend

# Финальный образ с nvidia-smi для монитора GPU (обе карты пробрасываются через deploy in compose).
FROM nvidia/cuda:12.4.0-runtime-ubuntu22.04
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates wget && rm -rf /var/lib/apt/lists/*
COPY --from=builder /admin-backend /admin-backend
EXPOSE 8080
ENTRYPOINT ["/admin-backend"]
