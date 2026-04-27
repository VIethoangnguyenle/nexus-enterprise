FROM golang:1.26-alpine AS builder
RUN apk add --no-cache git
WORKDIR /app
COPY go.mod go.sum ./
COPY proto/ proto/
COPY ngac/ ngac/
COPY pkg/ pkg/
COPY services/workspace/go.mod services/workspace/go.sum services/workspace/
WORKDIR /app/services/workspace
RUN go mod download
WORKDIR /app
COPY services/workspace/ services/workspace/
WORKDIR /app/services/workspace
RUN CGO_ENABLED=0 go build -o /workspace ./cmd/

FROM alpine:3.21
RUN apk add --no-cache ca-certificates wget && \
    wget -qO/bin/grpc_health_probe https://github.com/grpc-ecosystem/grpc-health-probe/releases/download/v0.4.35/grpc_health_probe-linux-amd64 && \
    chmod +x /bin/grpc_health_probe
COPY --from=builder /workspace /workspace
ENTRYPOINT ["/workspace"]
