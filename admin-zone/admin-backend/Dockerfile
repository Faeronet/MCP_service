FROM golang:1.23-alpine AS builder
WORKDIR /app
RUN apk add --no-cache git ca-certificates
COPY pkg/ ./pkg/
COPY admin-backend/go.mod admin-backend/go.sum ./admin-backend/
COPY admin-backend/ ./admin-backend/
WORKDIR /app/admin-backend
RUN go mod download && CGO_ENABLED=0 go build -o /admin-backend .

FROM nvidia/cuda:12.4.0-runtime-ubuntu22.04
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates wget && rm -rf /var/lib/apt/lists/*
COPY --from=builder /admin-backend /admin-backend
EXPOSE 8080
ENTRYPOINT ["/admin-backend"]
