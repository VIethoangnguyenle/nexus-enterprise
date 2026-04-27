## Why

The NGAC platform stores document files directly on Docker volumes via `os.WriteFile`, limiting file size to gRPC's 4MB default, preventing horizontal scaling (sticky volumes), and offering no presigned URL or CDN capability. Simultaneously, the frontend Nginx handles reverse proxying with hard-coded upstream addresses, and the custom Gateway service conflates infrastructure routing (CORS, TLS, rate-limiting) with business logic (REST→gRPC translation). Replacing file storage with MinIO and the Nginx/routing layer with Traefik separates infrastructure concerns, enables production-grade object storage with presigned uploads, and prepares the platform for multi-replica deployment.

## What Changes

- **Add MinIO** as S3-compatible object storage for all media (documents, future asset attachments)
- **Presigned upload flow**: clients get a signed PUT URL, upload directly to MinIO, then confirm — bypassing gRPC binary transfer
- **Presigned download URLs**: document downloads go through an access-checked URL endpoint that returns a time-limited signed GET URL
- **One bucket per workspace**: auto-created when workspace is provisioned
- **Add Traefik** as edge reverse proxy replacing Nginx's proxy role in the frontend container
- **Frontend Nginx** stripped to pure static file serving (SPA fallback only)
- **Traefik routes**: `/api/*` → Gateway, `/storage/*` → MinIO, `/api/ws` → Messaging WebSocket (direct, bypassing Gateway), `/*` → Frontend
- **Production TLS** via Traefik's Let's Encrypt ACME integration (not used in local dev)
- **BREAKING**: Document upload API changes from multipart `POST /api/workspaces/{id}/documents` with binary body to a two-step presigned URL flow
- **Internal service ports** no longer exposed to host — only Traefik ports (:80, :443) are published

## Capabilities

### New Capabilities
- `object-storage`: MinIO integration — bucket lifecycle, presigned upload/download URLs, S3 SDK usage, workspace bucket provisioning
- `edge-proxy`: Traefik configuration — Docker provider labels, routing rules, WebSocket support, CORS middleware, Let's Encrypt TLS for production
- `presigned-upload`: Two-step upload flow (get URL → upload → confirm) replacing inline binary gRPC transfer

### Modified Capabilities
_(none — no existing specs to modify)_

## Impact

- **docker-compose.yml**: Add `minio` and `traefik` services, remove host port mappings from internal services, add Traefik labels to all routable services
- **backend/proto/document/**: Add `GetUploadURL`, `ConfirmUpload`, `GetDownloadURL` RPCs; deprecate `bytes content` in `UploadRequest`
- **backend/services/document/**: Replace `os.WriteFile` with MinIO SDK (`minio-go`), add presigned URL generation
- **backend/services/workspace/**: Add MinIO bucket creation on workspace provisioning
- **backend/services/gateway/**: Add upload-url/confirm/download-url handlers, remove binary content relay
- **frontend/nginx.conf**: Remove `/api/` and `/api/ws` proxy blocks — Traefik handles routing
- **frontend/src/api/documents.ts**: New presigned upload flow (fetch URL → `PUT` to MinIO → confirm)
- **test_app.sh**: Add MinIO upload/download tests, update document flow assertions
- **New dependency**: `github.com/minio/minio-go/v7` in document and workspace services
