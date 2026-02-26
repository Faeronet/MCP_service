FROM golang:1.22-alpine AS builder
WORKDIR /app
RUN apk add --no-cache git ca-certificates

COPY go.mod go.sum* ./
COPY pkg/ ./pkg/
RUN mkdir -p ./admin-backend
COPY admin-backend/handler.go admin-backend/main_build.go admin-backend/metrics.go ./
RUN mv main_build.go admin-backend/main.go && mv handler.go admin-backend/ && mv metrics.go admin-backend/
RUN go mod tidy && go build -o /admin-backend ./admin-backend

FROM alpine:3.19
RUN apk add --no-cache ca-certificates
COPY --from=builder /admin-backend /admin-backend
EXPOSE 8080
ENTRYPOINT ["/admin-backend"]
