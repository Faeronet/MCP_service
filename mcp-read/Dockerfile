FROM golang:1.22-alpine AS builder
WORKDIR /app
RUN apk add --no-cache git ca-certificates

COPY go.mod go.sum* ./
COPY pkg/ ./pkg/
COPY mcp-read/ ./mcp-read/
RUN go mod download 2>/dev/null || true
RUN go build -o /mcp-read ./mcp-read

FROM alpine:3.19
RUN apk add --no-cache ca-certificates
COPY --from=builder /mcp-read /mcp-read
EXPOSE 8082
ENTRYPOINT ["/mcp-read"]
