# Build stage
FROM golang:1.25-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /ngac-server ./cmd/server

# Runtime stage
FROM alpine:3.21
RUN apk --no-cache add ca-certificates
WORKDIR /app
COPY --from=builder /ngac-server .
RUN mkdir -p /data/documents
EXPOSE 8080
CMD ["./ngac-server"]
