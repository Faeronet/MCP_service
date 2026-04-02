FROM golang:1.23-alpine AS build

WORKDIR /app

COPY go.mod ./
COPY modules ./modules
COPY cmd ./cmd

RUN go mod download

RUN CGO_ENABLED=0 go build -o /out/zone-agent ./cmd/zone-agent

FROM docker:27-cli

COPY --from=build /out/zone-agent /usr/local/bin/zone-agent

EXPOSE 19090

ENTRYPOINT ["/usr/local/bin/zone-agent"]

