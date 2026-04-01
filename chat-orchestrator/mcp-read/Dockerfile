FROM golang:1.23-alpine AS builder
WORKDIR /app
RUN apk add --no-cache git ca-certificates
COPY pkg/ ./pkg/
COPY mcp-read/go.mod mcp-read/go.sum ./mcp-read/
COPY mcp-read/ ./mcp-read/
WORKDIR /app/mcp-read
RUN go mod download && CGO_ENABLED=0 go build -o /mcp-read .

FROM alpine:3.19
RUN apk add --no-cache ca-certificates wget
COPY --from=builder /mcp-read /mcp-read
EXPOSE 8082
ENTRYPOINT ["/mcp-read"]
