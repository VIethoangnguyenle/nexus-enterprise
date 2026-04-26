FROM golang:1.26-alpine AS builder
RUN apk add --no-cache git
WORKDIR /app
# Copy root module (proto dependency)
COPY go.mod go.sum ./
COPY proto/ proto/
# Copy service module
COPY services/policy/go.mod services/policy/go.sum services/policy/
WORKDIR /app/services/policy
RUN go mod download
WORKDIR /app
COPY services/policy/ services/policy/
WORKDIR /app/services/policy
RUN CGO_ENABLED=0 go build -o /policy ./cmd/

FROM alpine:3.21
RUN apk add --no-cache ca-certificates wget && \
    wget -qO/bin/grpc_health_probe https://github.com/grpc-ecosystem/grpc-health-probe/releases/download/v0.4.35/grpc_health_probe-linux-amd64 && \
    chmod +x /bin/grpc_health_probe
COPY --from=builder /policy /policy
ENTRYPOINT ["/policy"]
