FROM golang:1.26-alpine AS builder
RUN apk add --no-cache git
WORKDIR /app
COPY go.mod go.sum ./
COPY proto/ proto/
COPY pkg/ pkg/
COPY ngac/ ngac/
COPY services/calendar/go.mod services/calendar/go.sum services/calendar/
WORKDIR /app/services/calendar
RUN go mod download
WORKDIR /app
COPY services/calendar/ services/calendar/
WORKDIR /app/services/calendar
RUN CGO_ENABLED=0 go build -o /calendar ./cmd/

FROM alpine:3.21
RUN apk add --no-cache ca-certificates wget && \
    wget -qO/bin/grpc_health_probe https://github.com/grpc-ecosystem/grpc-health-probe/releases/download/v0.4.35/grpc_health_probe-linux-amd64 && \
    chmod +x /bin/grpc_health_probe
COPY --from=builder /calendar /calendar
ENTRYPOINT ["/calendar"]
