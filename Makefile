# ============================================================================
# NGAC Platform — Top-level Makefile
# ============================================================================
# Usage:
#   make deploy              Build + start all services (Docker full stack)
#   make redeploy            Full cycle: down → build → up → health
#   make redeploy s=drive    Rebuild & restart only the drive service
#   make down                Stop all services
#   make restart s=redpanda  Restart a single service
#   make ps                  Show service status
#   make health              Wait for all services to be healthy
#   make logs                Tail all logs
#   make logs s=drive        Tail logs for one service
#   make build-check         Local Go build verification (no Docker)
#   make test                Run all Go tests
#   make test s=drive        Run tests for one service
#   make proto               Regenerate protobuf Go code
#
# Local Development:
#   make dev                 Start infra (Docker) + all services (native) + frontend
#   make dev-infra           Start only infrastructure (Docker)
#   make dev-stop            Stop all native services
#   make dev-logs            Tail all service logs
#   make dev-logs s=auth     Tail one service log
# ============================================================================

.PHONY: deploy redeploy down restart ps health logs \
        build-check test proto proto-install dev-frontend clean db-migrate help \
        dev dev-infra dev-stop dev-logs dev-all dev-restart dev-connect run stop

# ---------------------------------------------------------------------------
# Auto-detect docker compose command (v2 plugin vs v1 standalone)
# ---------------------------------------------------------------------------
COMPOSE := $(shell docker compose version >/dev/null 2>&1 && echo "docker compose" || echo "docker-compose")

# All backend services (order matters for build-check and dev startup)
SERVICES := policy auth workspace document messaging asset drive approval

# Service filter — use as: make redeploy s=drive
s ?=

# Dev environment files
DEV_ENV := .env.dev
DEV_PIDS := .dev-pids
DEV_LOGS := .dev-logs

# ---------------------------------------------------------------------------
# Deploy & Lifecycle (Docker full stack)
# ---------------------------------------------------------------------------

## Start all services (build if needed) — full Docker stack
deploy:
	@echo "▸ Building and starting all services..."
	$(COMPOSE) --profile app up -d --build
	@$(MAKE) --no-print-directory health
	@$(MAKE) --no-print-directory db-migrate

## Full redeploy cycle — stop, rebuild, start, verify
## Use s=<name> to target a single service
redeploy:
ifdef s
	@echo "▸ Redeploying service: $(s)"
	$(COMPOSE) --profile app up -d --build --force-recreate --no-deps $(s)
	@sleep 5
	@$(MAKE) --no-print-directory _check-service SVC=$(s)
else
	@echo "▸ Full redeploy — all services"
	$(COMPOSE) --profile app down --remove-orphans
	$(COMPOSE) --profile app up -d --build --force-recreate
	@$(MAKE) --no-print-directory health
endif

## Stop all services
down:
	$(COMPOSE) --profile app down --remove-orphans

## Restart a service without rebuilding — use: make restart s=redpanda
restart:
ifndef s
	@echo "Error: specify service with s=<name>"; exit 1
endif
	$(COMPOSE) restart $(s)
	@sleep 5
	@$(MAKE) --no-print-directory _check-service SVC=$(s)

# ---------------------------------------------------------------------------
# Observability
# ---------------------------------------------------------------------------

## Show container status
ps:
	$(COMPOSE) --profile app ps

## Tail logs (all or single service via s=)
logs:
ifdef s
	$(COMPOSE) logs -f --tail=100 $(s)
else
	$(COMPOSE) logs -f --tail=50
endif

## Wait for all services to pass health checks (timeout 90s)
health:
	@echo "▸ Waiting for services to be healthy..."
	@for i in $$(seq 1 18); do \
		UNHEALTHY=$$($(COMPOSE) --profile app ps 2>/dev/null | grep -cE "starting|unhealthy|Exit" || true); \
		if [ "$$UNHEALTHY" = "0" ]; then \
			echo "✓ All services healthy"; \
			$(COMPOSE) --profile app ps; \
			exit 0; \
		fi; \
		echo "  …waiting ($$i/18) — $$UNHEALTHY not ready"; \
		sleep 5; \
	done; \
	echo "⚠ Some services still not healthy after 90s:"; \
	$(COMPOSE) --profile app ps | grep -E "starting|unhealthy|Exit"; \
	exit 1

# Internal: check a single service health
_check-service:
	@echo "▸ Checking $(SVC)..."
	@for i in $$(seq 1 12); do \
		STATUS=$$($(COMPOSE) ps $(SVC) 2>/dev/null | grep -cE "Up.*healthy" || true); \
		if [ "$$STATUS" -ge 1 ]; then \
			echo "✓ $(SVC) is healthy"; \
			exit 0; \
		fi; \
		sleep 5; \
	done; \
	echo "⚠ $(SVC) not healthy after 60s"; \
	$(COMPOSE) logs --tail=20 $(SVC); \
	exit 1

# ---------------------------------------------------------------------------
# Local Development (native Go services + Docker infra)
# ---------------------------------------------------------------------------

## Start only infrastructure (postgres, redis, redpanda, minio)
dev-infra:
	@echo "▸ Starting infrastructure services..."
	$(COMPOSE) up -d
	@echo "▸ Waiting for infra to be healthy..."
	@for i in $$(seq 1 18); do \
		UNHEALTHY=$$($(COMPOSE) ps 2>/dev/null | grep -cE "starting|unhealthy|Exit" || true); \
		if [ "$$UNHEALTHY" = "0" ]; then \
			echo "✓ Infrastructure healthy"; \
			exit 0; \
		fi; \
		echo "  …waiting ($$i/18) — $$UNHEALTHY not ready"; \
		sleep 5; \
	done; \
	echo "⚠ Infrastructure not healthy after 90s"; exit 1
	@$(MAKE) --no-print-directory db-migrate

## Start full local dev: infra + native Go services + frontend
dev: dev-infra
	@if [ -f $(DEV_PIDS) ]; then \
		echo "⚠ Dev services may already be running ($(DEV_PIDS) exists)."; \
		echo "  Run 'make dev-stop' first, or delete $(DEV_PIDS) if stale."; \
		exit 1; \
	fi
	@mkdir -p $(DEV_LOGS)
	@echo "▸ Starting native Go services..."
	@# Source env and start each service
	@set -a && . ./$(DEV_ENV) && set +a && \
	\
	DATABASE_URL=$$DATABASE_URL \
	REDIS_URL=$$REDIS_URL_POLICY \
	KAFKA_BROKERS=$$KAFKA_BROKERS \
	GRPC_PORT=50051 \
	go run ./backend/services/policy/cmd/ > $(DEV_LOGS)/policy.log 2>&1 & echo $$! >> $(DEV_PIDS); \
	echo "  policy       → gRPC :50051"; \
	sleep 2; \
	\
	DATABASE_URL=$$DATABASE_URL \
	REDIS_URL=$$REDIS_URL_AUTH \
	POLICY_SERVICE_ADDR=$$POLICY_SERVICE_ADDR \
	WORKSPACE_SERVICE_ADDR=$$WORKSPACE_SERVICE_ADDR \
	MESSAGING_SERVICE_ADDR=$$MESSAGING_SERVICE_ADDR \
	JWT_SECRET=$$JWT_SECRET \
	GRPC_PORT=50052 \
	REST_PORT=$$AUTH_REST_PORT \
	go run ./backend/services/auth/cmd/ > $(DEV_LOGS)/auth.log 2>&1 & echo $$! >> $(DEV_PIDS); \
	echo "  auth         → gRPC :50052  REST :$$AUTH_REST_PORT"; \
	\
	DATABASE_URL=$$DATABASE_URL \
	POLICY_SERVICE_ADDR=$$POLICY_SERVICE_ADDR \
	DRIVE_SERVICE_ADDR=$$DRIVE_SERVICE_ADDR \
	MINIO_ENDPOINT=$$MINIO_ENDPOINT \
	MINIO_ACCESS_KEY=$$MINIO_ACCESS_KEY \
	MINIO_SECRET_KEY=$$MINIO_SECRET_KEY \
	MINIO_USE_SSL=$$MINIO_USE_SSL \
	JWT_SECRET=$$JWT_SECRET \
	GRPC_PORT=50053 \
	REST_PORT=$$WORKSPACE_REST_PORT \
	go run ./backend/services/workspace/cmd/ > $(DEV_LOGS)/workspace.log 2>&1 & echo $$! >> $(DEV_PIDS); \
	echo "  workspace    → gRPC :50053  REST :$$WORKSPACE_REST_PORT"; \
	\
	DATABASE_URL=$$DATABASE_URL \
	MINIO_ENDPOINT=$$MINIO_ENDPOINT \
	MINIO_ACCESS_KEY=$$MINIO_ACCESS_KEY \
	MINIO_SECRET_KEY=$$MINIO_SECRET_KEY \
	MINIO_USE_SSL=$$MINIO_USE_SSL \
	MINIO_PUBLIC_ENDPOINT=$$MINIO_PUBLIC_ENDPOINT \
	DRIVE_SERVICE_ADDR=$$DRIVE_SERVICE_ADDR \
	JWT_SECRET=$$JWT_SECRET \
	GRPC_PORT=50054 \
	REST_PORT=$$DOCUMENT_REST_PORT \
	go run ./backend/services/document/cmd/ > $(DEV_LOGS)/document.log 2>&1 & echo $$! >> $(DEV_PIDS); \
	echo "  document     → gRPC :50054  REST :$$DOCUMENT_REST_PORT"; \
	\
	DATABASE_URL=$$DATABASE_URL \
	REDIS_URL=$$REDIS_URL_MESSAGING \
	KAFKA_BROKERS=$$KAFKA_BROKERS \
	POLICY_SERVICE_ADDR=$$POLICY_SERVICE_ADDR \
	AUTH_SERVICE_ADDR=$$AUTH_SERVICE_ADDR \
	DRIVE_SERVICE_ADDR=$$DRIVE_SERVICE_ADDR \
	JWT_SECRET=$$JWT_SECRET \
	GRPC_PORT=50055 \
	WS_PORT=$$WS_PORT \
	REST_PORT=$$MESSAGING_REST_PORT \
	go run ./backend/services/messaging/cmd/ > $(DEV_LOGS)/messaging.log 2>&1 & echo $$! >> $(DEV_PIDS); \
	echo "  messaging    → gRPC :50055  REST :$$MESSAGING_REST_PORT  WS :$$WS_PORT"; \
	\
	DATABASE_URL=$$DATABASE_URL \
	POLICY_SERVICE_ADDR=$$POLICY_SERVICE_ADDR \
	KAFKA_BROKERS=$$KAFKA_BROKERS \
	JWT_SECRET=$$JWT_SECRET \
	GRPC_PORT=50056 \
	REST_PORT=$$ASSET_REST_PORT \
	go run ./backend/services/asset/cmd/ > $(DEV_LOGS)/asset.log 2>&1 & echo $$! >> $(DEV_PIDS); \
	echo "  asset        → gRPC :50056  REST :$$ASSET_REST_PORT"; \
	\
	DATABASE_URL=$$DATABASE_URL \
	POLICY_SERVICE_ADDR=$$POLICY_SERVICE_ADDR \
	POLICY_READ_SERVICE_ADDR=$$POLICY_READ_SERVICE_ADDR \
	DOCUMENT_SERVICE_ADDR=$$DOCUMENT_SERVICE_ADDR \
	JWT_SECRET=$$JWT_SECRET \
	GRPC_PORT=50057 \
	REST_PORT=$$DRIVE_REST_PORT \
	go run ./backend/services/drive/cmd/ > $(DEV_LOGS)/drive.log 2>&1 & echo $$! >> $(DEV_PIDS); \
	echo "  drive        → gRPC :50057  REST :$$DRIVE_REST_PORT"; \
	\
	DATABASE_URL=$$DATABASE_URL \
	POLICY_ADDR=$$POLICY_SERVICE_ADDR \
	JWT_SECRET=$$JWT_SECRET \
	GRPC_PORT=50058 \
	REST_PORT=$$APPROVAL_REST_PORT \
	go run ./backend/services/approval/cmd/ > $(DEV_LOGS)/approval.log 2>&1 & echo $$! >> $(DEV_PIDS); \
	echo "  approval     → gRPC :50058  REST :$$APPROVAL_REST_PORT"
	@echo ""
	@echo "▸ Starting frontend dev server..."
	@cd frontend && VITE_DEV_MODE=true npm run dev -- --port 5173 --host > ../$(DEV_LOGS)/frontend.log 2>&1 & echo $$! >> $(DEV_PIDS)
	@echo "  frontend     → http://localhost:5173"
	@echo ""
	@echo "✓ All services started. PIDs in $(DEV_PIDS), logs in $(DEV_LOGS)/"
	@echo "  make dev-logs          — tail all logs"
	@echo "  make dev-logs s=auth   — tail one service"
	@echo "  make dev-stop          — stop all"

# Overmind binary (installed via: go install github.com/DarthSim/overmind/v2@latest)
OVERMIND := $(shell /usr/local/go/bin/go env GOPATH)/bin/overmind

## Start all services in foreground — logs stream to terminal, Ctrl+C stops all
run: dev-infra
	@command -v tmux >/dev/null 2>&1 || { echo "✗ tmux required: sudo apt install tmux"; exit 1; }
	@test -x $(OVERMIND) || { echo "✗ overmind not found at $(OVERMIND)"; echo "  Install: /usr/local/go/bin/go install github.com/DarthSim/overmind/v2@latest"; exit 1; }
	@echo ""
	@echo "▸ Starting all services (foreground)..."
	@echo "  Ctrl+C — stop all"
	@echo ""
	$(OVERMIND) start -f Procfile.dev

## Stop everything — overmind, orphan processes, stale sockets
stop:
	@echo "▸ Stopping all dev services..."
	@# 1. Graceful overmind stop (if running)
	@if [ -S .overmind.sock ]; then \
		echo "  overmind stop..."; \
		$(OVERMIND) stop 2>/dev/null || true; \
		sleep 1; \
	fi
	@# 2. Kill stale tmux session
	@if tmux has-session -t ngac 2>/dev/null; then \
		echo "  killing tmux session 'ngac'..."; \
		tmux kill-session -t ngac 2>/dev/null || true; \
	fi
	@# 3. Remove stale socket
	@rm -f .overmind.sock
	@# 4. Kill orphan Go service processes
	@PIDS=$$(pgrep -f 'go run \./backend/services/' 2>/dev/null || true); \
	if [ -n "$$PIDS" ]; then \
		echo "  killing orphan Go processes: $$PIDS"; \
		echo "$$PIDS" | xargs kill 2>/dev/null || true; \
	fi
	@# 5. Kill compiled Go service binaries (go run creates temp binaries)
	@PIDS=$$(pgrep -f '/tmp/go-build.*/exe/' 2>/dev/null || true); \
	if [ -n "$$PIDS" ]; then \
		echo "  killing Go temp binaries: $$PIDS"; \
		echo "$$PIDS" | xargs kill 2>/dev/null || true; \
	fi
	@# 6. Kill frontend dev server
	@PIDS=$$(pgrep -f 'vite.*--port 5173' 2>/dev/null || true); \
	if [ -n "$$PIDS" ]; then \
		echo "  killing frontend dev server: $$PIDS"; \
		echo "$$PIDS" | xargs kill 2>/dev/null || true; \
	fi
	@# 7. Clean dev-pids (from 'make dev' mode)
	@if [ -f $(DEV_PIDS) ]; then \
		while read pid; do \
			kill $$pid 2>/dev/null && echo "  killed PID $$pid (dev-pids)" || true; \
		done < $(DEV_PIDS); \
		rm -f $(DEV_PIDS); \
	fi
	@echo "✓ All dev services stopped"

## Start all services in background (daemon mode)
dev-all: dev-infra
	@command -v tmux >/dev/null 2>&1 || { echo "✗ tmux required: sudo apt install tmux"; exit 1; }
	@test -x $(OVERMIND) || { echo "✗ overmind not found at $(OVERMIND)"; exit 1; }
	@echo ""
	@echo "▸ Starting all services with overmind (background)..."
	@echo ""
	$(OVERMIND) start -f Procfile.dev --daemonize
	@echo "✓ All services started in background"
	@echo ""
	@echo "  overmind echo          — stream all logs"
	@echo "  overmind restart auth  — restart one service"
	@echo "  overmind connect auth  — attach to one service's terminal"
	@echo "  overmind stop          — stop all services"

## Restart a single overmind-managed service — use: make dev-restart s=auth
dev-restart:
ifndef s
	@echo "Error: specify service with s=<name> (e.g., make dev-restart s=auth)"; exit 1
endif
	$(OVERMIND) restart $(s)

## Connect to a service's interactive terminal — use: make dev-connect s=auth
dev-connect:
ifndef s
	@echo "Error: specify service with s=<name> (e.g., make dev-connect s=auth)"; exit 1
endif
	$(OVERMIND) connect $(s)

## Stop all native dev services
dev-stop:
	@if [ ! -f $(DEV_PIDS) ]; then \
		echo "No $(DEV_PIDS) file found — nothing to stop."; \
		exit 0; \
	fi
	@echo "▸ Stopping dev services..."
	@while read pid; do \
		if kill -0 $$pid 2>/dev/null; then \
			kill $$pid 2>/dev/null && echo "  killed PID $$pid"; \
		else \
			echo "  PID $$pid already exited (stale)"; \
		fi; \
	done < $(DEV_PIDS)
	@rm -f $(DEV_PIDS)
	@echo "✓ All dev services stopped"

## Tail dev service logs
dev-logs:
ifdef s
	@tail -f $(DEV_LOGS)/$(s).log
else
	@tail -f $(DEV_LOGS)/*.log
endif

# ---------------------------------------------------------------------------
# Build & Test (local, no Docker)
# ---------------------------------------------------------------------------

## Verify all services compile locally
build-check:
	@echo "▸ Building all services locally..."
	@FAIL=0; \
	for svc in $(SERVICES); do \
		printf "  %-12s" "$$svc"; \
		if cd backend/services/$$svc && go build -o /dev/null ./cmd/ 2>/tmp/ngac-build-$$svc.err; then \
			echo "✓"; \
		else \
			echo "✗"; \
			cat /tmp/ngac-build-$$svc.err; \
			FAIL=1; \
		fi; \
		cd $(CURDIR); \
	done; \
	if [ "$$FAIL" = "1" ]; then echo "⚠ Build failures detected"; exit 1; fi; \
	echo "✓ All services build OK"

## Run Go tests (all or single service via s=)
test:
ifdef s
	@echo "▸ Testing $(s)..."
	cd backend/services/$(s) && go test -v -count=1 -timeout 60s ./...
else
	@echo "▸ Testing all services..."
	@for svc in $(SERVICES); do \
		echo "=== $$svc ==="; \
		cd $(CURDIR)/backend/services/$$svc && go test -count=1 -timeout 60s ./... 2>&1 | tail -3; \
	done
endif

# ---------------------------------------------------------------------------
# Proto & Frontend
# ---------------------------------------------------------------------------

## Install protoc Go plugins
proto-install:
	$(MAKE) -C backend proto-install

## Regenerate protobuf Go code
proto:
	$(MAKE) -C backend proto

## Start frontend dev server (standalone, proxies to Docker)
dev-frontend:
	cd frontend && npm run dev

# ---------------------------------------------------------------------------
# Cleanup
# ---------------------------------------------------------------------------

## Re-apply init.sql schema on running DB (safe — uses IF NOT EXISTS)
db-migrate:
	@echo "▸ Applying schema migrations..."
	docker exec -i $$($(COMPOSE) ps -q postgres) psql -U ngac < data/init.sql
	@echo "✓ Schema up to date"

## Remove build artifacts
clean:
	$(MAKE) -C backend clean

## Nuclear option — remove all containers, volumes, and images
nuke:
	@echo "⚠ This will DELETE all data (postgres, redis, minio)!"
	@read -p "Type 'yes' to confirm: " CONFIRM; \
	if [ "$$CONFIRM" = "yes" ]; then \
		$(COMPOSE) --profile app down -v --rmi local --remove-orphans; \
		echo "✓ Everything removed"; \
	else \
		echo "Cancelled"; \
	fi

# ---------------------------------------------------------------------------
# Help
# ---------------------------------------------------------------------------

## Show this help
help:
	@echo "NGAC Platform — Available commands:"
	@echo ""
	@echo "  LIFECYCLE (Docker full stack)"
	@echo "    make deploy              Build + start all (Docker)"
	@echo "    make redeploy            Full cycle: down → build → up → health"
	@echo "    make redeploy s=drive    Rebuild only one service"
	@echo "    make down                Stop all"
	@echo "    make restart s=redpanda  Restart one service"
	@echo ""
	@echo "  LOCAL DEV (native Go + Docker infra)"
	@echo "    make dev                 Start infra + all services (native)"
	@echo "    make dev-infra           Start only infrastructure (Docker)"
	@echo "    make dev-stop            Stop all native services"
	@echo "    make dev-logs            Tail all service logs"
	@echo "    make dev-logs s=auth     Tail one service log"
	@echo ""
	@echo "  OBSERVE"
	@echo "    make ps                  Service status"
	@echo "    make health              Wait for healthy (90s timeout)"
	@echo "    make logs                Tail all logs (Docker)"
	@echo "    make logs s=gateway      Tail one service (Docker)"
	@echo ""
	@echo "  DEVELOP"
	@echo "    make build-check         Verify all services compile"
	@echo "    make test                Run all tests"
	@echo "    make test s=drive        Test one service"
	@echo "    make proto               Regen protobuf code"
	@echo "    make dev-frontend        Start Vite dev server (standalone)"
	@echo ""
	@echo "  LOCAL DEV (overmind — recommended)"
	@echo "    make run                 Start infra + all services (overmind, foreground)"
	@echo "    make stop                Stop everything (overmind, processes, sockets)"
	@echo "    make dev-all             Start in background (overmind daemon)"
	@echo "    make dev-restart s=auth  Restart one service (overmind)"
	@echo "    make dev-connect s=auth  Attach to one service terminal (overmind)"
	@echo ""
	@echo "  DATABASE"
	@echo "    make db-migrate          Apply schema to running DB"
	@echo ""
	@echo "  DANGER"
	@echo "    make nuke                Delete all containers + volumes"

