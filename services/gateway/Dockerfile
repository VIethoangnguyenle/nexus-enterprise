FROM golang:1.26-alpine AS builder
RUN apk add --no-cache git
WORKDIR /app
COPY go.mod go.sum ./
COPY proto/ proto/
COPY services/gateway/go.mod services/gateway/go.sum services/gateway/
WORKDIR /app/services/gateway
RUN go mod download
WORKDIR /app
COPY services/gateway/ services/gateway/
WORKDIR /app/services/gateway
RUN CGO_ENABLED=0 go build -o /gateway ./cmd/

FROM alpine:3.21
RUN apk add --no-cache ca-certificates
COPY --from=builder /gateway /gateway
ENTRYPOINT ["/gateway"]
