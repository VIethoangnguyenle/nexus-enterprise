.PHONY: proto proto-install build clean dev

# Delegate to backend Makefile
proto-install:
	$(MAKE) -C backend proto-install

proto:
	$(MAKE) -C backend proto

build:
	$(MAKE) -C backend build

clean:
	$(MAKE) -C backend clean

# Frontend dev
dev-frontend:
	cd frontend && npm run dev

# Docker
up:
	docker compose up -d

down:
	docker compose down
