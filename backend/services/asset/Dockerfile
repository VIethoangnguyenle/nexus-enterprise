FROM golang:1.26-alpine AS builder
RUN apk add --no-cache git
WORKDIR /app
COPY go.mod go.sum ./
COPY proto/ proto/
COPY services/asset/go.mod services/asset/go.sum services/asset/
WORKDIR /app/services/asset
RUN go mod download
WORKDIR /app
COPY services/asset/ services/asset/
WORKDIR /app/services/asset
RUN CGO_ENABLED=0 go build -o /asset ./cmd/

FROM alpine:3.21
RUN apk add --no-cache ca-certificates wget && \
    wget -qO/bin/grpc_health_probe https://github.com/grpc-ecosystem/grpc-health-probe/releases/download/v0.4.35/grpc_health_probe-linux-amd64 && \
    chmod +x /bin/grpc_health_probe
COPY --from=builder /asset /asset
ENTRYPOINT ["/asset"]
