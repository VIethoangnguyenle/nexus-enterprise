module ngac-platform/services/gateway

go 1.25.0

replace ngac-platform => ../..

require (
	github.com/go-chi/chi/v5 v5.2.5
	github.com/go-chi/cors v1.2.2
	github.com/golang-jwt/jwt/v5 v5.3.1
	github.com/gorilla/websocket v1.5.3
	google.golang.org/grpc v1.72.0
	google.golang.org/protobuf v1.36.6
	ngac-platform v0.0.0
)

require (
	golang.org/x/net v0.35.0 // indirect
	golang.org/x/sys v0.30.0 // indirect
	golang.org/x/text v0.36.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20250218202821-56aae31c358a // indirect
)
