FROM golang:1.26-alpine AS builder
RUN apk add --no-cache git
WORKDIR /app
COPY go.mod go.sum ./
COPY proto/ proto/
COPY services/auth/go.mod services/auth/go.sum services/auth/
WORKDIR /app/services/auth
RUN go mod download
WORKDIR /app
COPY services/auth/ services/auth/
WORKDIR /app/services/auth
RUN CGO_ENABLED=0 go build -o /auth ./cmd/

FROM alpine:3.21
RUN apk add --no-cache ca-certificates wget && \
    wget -qO/bin/grpc_health_probe https://github.com/grpc-ecosystem/grpc-health-probe/releases/download/v0.4.35/grpc_health_probe-linux-amd64 && \
    chmod +x /bin/grpc_health_probe
COPY --from=builder /auth /auth
ENTRYPOINT ["/auth"]
