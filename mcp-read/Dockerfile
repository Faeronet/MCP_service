FROM golang:1.23-alpine AS builder
WORKDIR /app
RUN apk add --no-cache git ca-certificates

COPY go.mod go.sum* ./
COPY pkg/ ./pkg/
COPY mcp-read/ ./mcp-read/
# Убрать случайный ведущий backslash в main.go (illegal character U+005C)
RUN sed -i '1s/^\\//' mcp-read/main.go 2>/dev/null || true
RUN go mod tidy && go build -o /mcp-read ./mcp-read

FROM alpine:3.19
RUN apk add --no-cache ca-certificates
COPY --from=builder /mcp-read /mcp-read
EXPOSE 8082
ENTRYPOINT ["/mcp-read"]
