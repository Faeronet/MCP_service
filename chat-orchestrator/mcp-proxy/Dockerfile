FROM golang:1.23-alpine AS builder
WORKDIR /app
RUN apk add --no-cache git ca-certificates
COPY pkg/ ./pkg/
COPY mcp-proxy/go.mod mcp-proxy/go.sum ./mcp-proxy/
COPY mcp-proxy/ ./mcp-proxy/
WORKDIR /app/mcp-proxy
RUN go mod download && CGO_ENABLED=0 go build -o /mcp-proxy .

FROM alpine:3.19
RUN apk add --no-cache ca-certificates wget tzdata
COPY --from=builder /mcp-proxy /mcp-proxy
COPY mcp-proxy/angels-img /angels-img
EXPOSE 8083
ENTRYPOINT ["/mcp-proxy"]
