## ADDED Requirements

### Requirement: Traefik as edge reverse proxy
The system SHALL use Traefik as the single entry point for all HTTP/WebSocket traffic, replacing Nginx's reverse proxy role.

#### Scenario: All traffic enters via Traefik
- **WHEN** a client makes any HTTP request
- **THEN** the request is received by Traefik on port 80 (dev) or 443 (production)
- **THEN** Traefik routes to the appropriate backend service

#### Scenario: Internal service ports not exposed to host
- **WHEN** `docker-compose up` is executed
- **THEN** only Traefik ports (80, 443) and Traefik dashboard (8082) are published to the host
- **THEN** internal gRPC ports (50051-50056) are NOT mapped to host

### Requirement: Route API traffic to Gateway
The system SHALL route all requests matching `PathPrefix(/api)` (except `/api/ws`) to the Gateway service.

#### Scenario: REST API requests reach Gateway
- **WHEN** client sends `POST /api/workspaces`
- **THEN** Traefik forwards to `gateway:8080`
- **THEN** Gateway processes the request normally

### Requirement: Route WebSocket directly to Messaging
The system SHALL route WebSocket upgrade requests at `/api/ws` directly to the Messaging service, bypassing Gateway.

#### Scenario: WebSocket connects without Gateway hop
- **WHEN** client opens WebSocket at `/api/ws?token=xxx`
- **THEN** Traefik forwards directly to `messaging:8081`
- **THEN** WebSocket connection is established with one fewer network hop

### Requirement: Route storage traffic to MinIO
The system SHALL route requests matching `PathPrefix(/storage)` to the MinIO S3 API for presigned URL uploads/downloads.

#### Scenario: Presigned upload reaches MinIO
- **WHEN** client sends `PUT /storage/ws-xxx/documents/doc-id/file.pdf` with presigned query params
- **THEN** Traefik forwards to `minio:9000`
- **THEN** MinIO validates the signature and stores the object

### Requirement: Serve frontend static files
The system SHALL route all non-API, non-storage requests to the frontend service for SPA serving.

#### Scenario: Frontend page loads
- **WHEN** client navigates to `/` or `/workspaces/123`
- **THEN** Traefik forwards to `frontend:80`
- **THEN** Nginx serves `index.html` (SPA fallback)

### Requirement: CORS middleware on Traefik
The system SHALL configure CORS headers at the Traefik level for API and storage routes.

#### Scenario: CORS preflight succeeds
- **WHEN** browser sends OPTIONS preflight to `/api/workspaces`
- **THEN** Traefik responds with `Access-Control-Allow-Origin: *` and allowed methods/headers

### Requirement: Docker provider with labels
The system SHALL configure all Traefik routing via Docker labels on service definitions in `docker-compose.yml`, not via static config files.

#### Scenario: New service auto-discovered
- **WHEN** a new service container starts with Traefik labels
- **THEN** Traefik automatically creates routes without restart

### Requirement: Production TLS with Let's Encrypt
The system SHALL support automatic TLS certificate provisioning via Traefik's ACME resolver in production deployments.

#### Scenario: TLS enabled in production
- **WHEN** `TRAEFIK_ACME_EMAIL` environment variable is set
- **THEN** Traefik obtains Let's Encrypt certificates for the configured domain
- **THEN** HTTP traffic is redirected to HTTPS

#### Scenario: TLS disabled in local dev
- **WHEN** `TRAEFIK_ACME_EMAIL` is not set
- **THEN** Traefik serves HTTP only on port 80
- **THEN** no certificate errors in local development

### Requirement: Frontend Nginx stripped to static serving
The system SHALL remove all `proxy_pass` directives from the frontend Nginx config, keeping only SPA fallback and static asset caching.

#### Scenario: Nginx no longer proxies API requests
- **WHEN** frontend Nginx receives a request to `/api/workspaces`
- **THEN** Nginx returns the SPA `index.html` (not a proxy response)
- **THEN** the browser's fetch request goes through Traefik instead
