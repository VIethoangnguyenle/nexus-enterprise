FROM golang:1.26-alpine AS builder
RUN apk add --no-cache git
WORKDIR /app
COPY go.mod go.sum ./
COPY proto/ proto/
COPY pkg/ pkg/
COPY ngac/ ngac/
COPY services/document/go.mod services/document/go.sum services/document/
WORKDIR /app/services/document
RUN go mod download
WORKDIR /app
COPY services/document/ services/document/
WORKDIR /app/services/document
RUN CGO_ENABLED=0 go build -o /document ./cmd/

FROM alpine:3.21
RUN apk add --no-cache ca-certificates wget && \
    wget -qO/bin/grpc_health_probe https://github.com/grpc-ecosystem/grpc-health-probe/releases/download/v0.4.35/grpc_health_probe-linux-amd64 && \
    chmod +x /bin/grpc_health_probe
COPY --from=builder /document /document
ENTRYPOINT ["/document"]
