FROM golang:1.23-alpine AS builder
WORKDIR /app
RUN apk add --no-cache git ca-certificates

COPY go.mod go.sum* ./
COPY pkg/ ./pkg/
COPY mcp-proxy/ ./mcp-proxy/
RUN go mod tidy && go build -o /mcp-proxy ./mcp-proxy

FROM alpine:3.19
RUN apk add --no-cache ca-certificates wget
COPY --from=builder /mcp-proxy /mcp-proxy
EXPOSE 8083
ENTRYPOINT ["/mcp-proxy"]
