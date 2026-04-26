.PHONY: proto proto-install build clean

# Install protoc-gen-go and protoc-gen-go-grpc
proto-install:
	go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
	go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest

# Generate Go code from proto files
proto:
	@echo "Generating proto files..."
	protoc -I. -I$(HOME)/.local/include --go_out=. --go_opt=paths=source_relative \
		--go-grpc_out=. --go-grpc_opt=paths=source_relative \
		proto/policy/policy.proto
	protoc -I. -I$(HOME)/.local/include --go_out=. --go_opt=paths=source_relative \
		--go-grpc_out=. --go-grpc_opt=paths=source_relative \
		proto/auth/auth.proto
	protoc -I. -I$(HOME)/.local/include --go_out=. --go_opt=paths=source_relative \
		--go-grpc_out=. --go-grpc_opt=paths=source_relative \
		proto/workspace/workspace.proto
	protoc -I. -I$(HOME)/.local/include --go_out=. --go_opt=paths=source_relative \
		--go-grpc_out=. --go-grpc_opt=paths=source_relative \
		proto/document/document.proto
	protoc -I. -I$(HOME)/.local/include --go_out=. --go_opt=paths=source_relative \
		--go-grpc_out=. --go-grpc_opt=paths=source_relative \
		proto/messaging/messaging.proto
	protoc -I. -I$(HOME)/.local/include --go_out=. --go_opt=paths=source_relative \
		--go-grpc_out=. --go-grpc_opt=paths=source_relative \
		proto/asset/asset.proto
	protoc -I. -I$(HOME)/.local/include --go_out=. --go_opt=paths=source_relative \
		--go-grpc_out=. --go-grpc_opt=paths=source_relative \
		proto/asset/asset_request.proto
	@echo "Proto generation complete!"

# Build all services
build:
	@for svc in gateway auth policy workspace document messaging asset; do \
		echo "Building $$svc..."; \
		cd services/$$svc && go build -o ../../bin/$$svc ./cmd/ && cd ../..; \
	done

# Clean generated files
clean:
	rm -rf bin/
	find proto -name "*.pb.go" -delete
