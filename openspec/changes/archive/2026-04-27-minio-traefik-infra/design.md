## Context

The NGAC platform currently stores document files on local Docker volumes using `os.WriteFile` in the Document Service. The frontend Nginx container serves dual duty: serving static SPA files and reverse-proxying API/WebSocket traffic to the Gateway. The Gateway service handles both infrastructure concerns (CORS, logging) and business logic (REST→gRPC translation, JWT claim injection).

Current limitations:
- **File storage**: 4MB gRPC message limit, no presigned URLs, no CDN, Docker volume is non-scalable
- **Routing**: Nginx proxy config is manual and tightly coupled to service addresses, no native Let's Encrypt, no service discovery
- **WebSocket**: Double-proxied (Nginx → Gateway → Messaging), adding latency

## Goals / Non-Goals

**Goals:**
- Replace local file storage with MinIO (S3-compatible) using presigned upload/download URLs
- Auto-create one MinIO bucket per workspace on provisioning
- Replace Nginx reverse proxy with Traefik as edge proxy (Docker provider, label-based routing)
- Route WebSocket traffic directly from Traefik to Messaging (bypass Gateway)
- Prepare production TLS via Traefik ACME (Let's Encrypt)
- All 52 existing tests continue to pass + new MinIO-specific tests
- Frontend upload flow works via presigned URL

**Non-Goals:**
- CDN integration (future work)
- Replacing the Gateway's REST→gRPC translation layer (Gateway remains as BFF)
- Multi-region MinIO replication
- Asset service file attachments (phase 2)
- gRPC-Web or grpc-gateway migration
- Authentication changes — JWT validation stays in Gateway middleware

## Decisions

### D1: Traefik as edge proxy, Gateway stays as BFF

**Decision**: Add Traefik in front of all services. Keep the Go Gateway for REST→gRPC translation.

**Alternatives considered**:
- *Replace Gateway entirely with grpc-gateway*: Requires proto annotations on every RPC, moves auth into interceptors. Too much churn for this change.
- *Replace Gateway with gRPC-Web*: Requires full frontend rewrite of API layer. Not justified yet.

**Rationale**: Traefik handles what Nginx currently does (routing, TLS, CORS) but with Docker-native service discovery. Gateway keeps its BFF role — the ~60 handler functions that translate REST→gRPC stay. This is the lowest-risk path.

### D2: Presigned URL upload (not proxy-through)

**Decision**: Clients upload directly to MinIO via presigned PUT URL. Gateway only brokers the URL, never touches the binary.

**Alternatives considered**:
- *Proxy upload through Gateway*: Simpler but reintroduces the gRPC 4MB limit and wastes Gateway bandwidth.
- *Streaming gRPC upload*: Complex, requires proto changes to streaming RPCs.

**Rationale**: Presigned URLs eliminate binary transfer through the service mesh entirely. Client → MinIO is direct. The three-step flow (get-url → upload → confirm) is standard S3 pattern.

### D3: One bucket per workspace

**Decision**: Each workspace gets a MinIO bucket named `ws-{workspace_id}`.

**Alternatives considered**:
- *Single global bucket with path prefixes*: Simpler but loses bucket-level IAM policies and makes cleanup harder.
- *Bucket per document type*: Over-segmented, no clear benefit.

**Rationale**: Bucket-per-workspace provides natural isolation, simplifies workspace deletion (drop bucket), and allows future per-workspace storage quotas.

### D4: MinIO SDK (minio-go) in Document + Workspace services

**Decision**: Use `github.com/minio/minio-go/v7` directly. Document Service generates presigned URLs and verifies uploads. Workspace Service creates buckets.

**Alternatives considered**:
- *Shared storage service*: Adds a new microservice just for S3 calls — overhead not justified.
- *AWS SDK*: Heavier, unnecessary since we control the MinIO instance.

**Rationale**: minio-go is lightweight, purpose-built, and the services already have different responsibilities (workspace = lifecycle, document = content).

### D5: Traefik Docker provider with labels

**Decision**: Configure routing via Docker labels on each service in `docker-compose.yml`. No static config files.

**Rationale**: Labels are co-located with the service definition, making routing changes self-documenting. Traefik auto-discovers services on container start.

### D6: Frontend Nginx becomes static-only

**Decision**: Strip all `proxy_pass` directives from `nginx.conf`. Nginx only serves SPA static files with `try_files` fallback. Traefik handles all routing.

**Rationale**: Eliminates the double-proxy problem. Frontend container becomes purely a static file server.

## Risks / Trade-offs

- **[Risk] MinIO availability becomes critical** → MinIO runs as a single container in dev; production should use distributed mode or managed S3. Healthcheck ensures dependent services wait.
- **[Risk] Presigned URL expiry** → URLs expire after 5 minutes. Client must upload within window. Frontend shows clear error if expired.
- **[Risk] Orphaned objects in MinIO** → If client uploads but never confirms, objects accumulate. → Mitigation: MinIO lifecycle rules auto-delete objects without DB records after 24h.
- **[Risk] Traefik misconfiguration blocks all traffic** → All services behind single proxy. → Mitigation: Traefik dashboard on `:8082` for debugging; healthcheck on Traefik itself.
- **[Trade-off] Two-step upload is more complex for frontend** → But eliminates binary transfer through gRPC stack and removes 4MB limit.
- **[Trade-off] Internal service ports no longer accessible from host** → Debugging requires `docker exec` or Traefik dashboard. → Acceptable for production parity.

## Migration Plan

1. Add MinIO + Traefik to docker-compose (existing services unchanged initially)
2. Update proto + Document Service for presigned URL flow
3. Add bucket creation to Workspace Service
4. Update Gateway handlers for new upload endpoints
5. Add Traefik labels to all services, remove host port mappings
6. Strip Nginx proxy config
7. Update frontend upload flow
8. Update test_app.sh
9. Run full 52+ test suite

**Rollback**: Revert docker-compose.yml to restore Nginx proxy and direct port mappings. Document Service still has DB records; only object storage location changes.
