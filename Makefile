# ============================================================================
# NGAC Platform — Top-level Makefile
# ============================================================================
# Usage:
#   make deploy              Build + start all services
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
# ============================================================================

.PHONY: deploy redeploy down restart ps health logs \
        build-check test proto proto-install dev-frontend clean db-migrate help

# ---------------------------------------------------------------------------
# Auto-detect docker compose command (v2 plugin vs v1 standalone)
# ---------------------------------------------------------------------------
COMPOSE := $(shell docker compose version >/dev/null 2>&1 && echo "docker compose" || echo "docker-compose")

# All backend services (order matters for build-check)
SERVICES := policy auth workspace document messaging asset drive gateway

# Service filter — use as: make redeploy s=drive
s ?=

# ---------------------------------------------------------------------------
# Deploy & Lifecycle
# ---------------------------------------------------------------------------

## Start all services (build if needed)
deploy:
	@echo "▸ Building and starting all services..."
	$(COMPOSE) up -d --build
	@$(MAKE) --no-print-directory health
	@$(MAKE) --no-print-directory db-migrate

## Full redeploy cycle — stop, rebuild, start, verify
## Use s=<name> to target a single service
redeploy:
ifdef s
	@echo "▸ Redeploying service: $(s)"
	$(COMPOSE) up -d --build --force-recreate --no-deps $(s)
	@sleep 5
	@$(MAKE) --no-print-directory _check-service SVC=$(s)
else
	@echo "▸ Full redeploy — all services"
	$(COMPOSE) down --remove-orphans
	$(COMPOSE) up -d --build --force-recreate
	@$(MAKE) --no-print-directory health
endif

## Stop all services
down:
	$(COMPOSE) down --remove-orphans

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
	$(COMPOSE) ps

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
		UNHEALTHY=$$($(COMPOSE) ps 2>/dev/null | grep -cE "starting|unhealthy|Exit" || true); \
		if [ "$$UNHEALTHY" = "0" ]; then \
			echo "✓ All services healthy"; \
			$(COMPOSE) ps; \
			exit 0; \
		fi; \
		echo "  …waiting ($$i/18) — $$UNHEALTHY not ready"; \
		sleep 5; \
	done; \
	echo "⚠ Some services still not healthy after 90s:"; \
	$(COMPOSE) ps | grep -E "starting|unhealthy|Exit"; \
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

## Start frontend dev server
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
		$(COMPOSE) down -v --rmi local --remove-orphans; \
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
	@echo "  LIFECYCLE"
	@echo "    make deploy              Build + start all"
	@echo "    make redeploy            Full cycle: down → build → up → health"
	@echo "    make redeploy s=drive    Rebuild only one service"
	@echo "    make down                Stop all"
	@echo "    make restart s=redpanda  Restart one service"
	@echo ""
	@echo "  OBSERVE"
	@echo "    make ps                  Service status"
	@echo "    make health              Wait for healthy (90s timeout)"
	@echo "    make logs                Tail all logs"
	@echo "    make logs s=gateway      Tail one service"
	@echo ""
	@echo "  DEVELOP"
	@echo "    make build-check         Verify all services compile"
	@echo "    make test                Run all tests"
	@echo "    make test s=drive        Test one service"
	@echo "    make proto               Regen protobuf code"
	@echo "    make dev-frontend        Start Vite dev server"
	@echo ""
	@echo "  DATABASE"
	@echo "    make db-migrate          Apply schema to running DB"
	@echo ""
	@echo "  DANGER"
	@echo "    make nuke                Delete all containers + volumes"
