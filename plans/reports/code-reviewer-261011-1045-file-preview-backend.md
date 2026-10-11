# Code review: file-preview backend (uncommitted)

Scope: `backend/services/drive/**` (preview.go, preview_kind.go, gotenberg/, rest route, main.go, go.mod, tests),
`docker-compose.yml`, `deploy/docker-compose.prod.yml`, `docs/deployment.md`, `docs/specs/README.md`, spec `file-preview`.
Frontend and messaging ignored.

Checks run: `go vet ./...` clean; `go test -count=1 ./...` (drive, real DB) 210 passed; `make check-layering check-ngac` ok;
`deploy/check-compose-env.sh` ok.

## Verdict
Authorization, layering and cache keying are right. Two problems block prod: buffering the PDF in memory can OOM-kill
drive, which has a 128M limit, and storage I/O goes out over the public URL (hairpin through Traefik). Fix these two
before deploy.

## Critical
None.

## High

**H1. Drive can be OOM-killed by one document, and stays in a crash loop.**
`gotenberg/client.go:21,70` reads up to `maxPDFBytes = 200 MiB` into memory with `io.ReadAll`, which grows by doubling.
Prod drive is `limits.memory: 128M`, `GOMEMLIMIT: 100MiB` (`deploy/docker-compose.prod.yml:456` and below). The source cap
(50 MB) does not cap the output. A 40 MB pptx full of images commonly renders to a PDF of 60–150 MB.
Scenario: a member uploads such a deck and opens it. The read grows past 128M and the kernel kills drive, so every drive
endpoint fails for every tenant. Failures are deliberately not remembered (spec). The next viewer, or a retry, converts
again and kills it again. Nothing limits concurrent conversions either: N different files mean N buffers (`preview.go:189`
singleflight dedups per key only).
Fix:
- Spool the Gotenberg response to `os.CreateTemp` (or stream it into the PUT with a known length) instead of `ReadAll`.
- Cap `maxPDFBytes` well below the container limit, or derive it from config.
- Add a process-wide semaphore (1–2 concurrent conversions) around `convert`. Return `conversion_unavailable` or `failed`
  when the semaphore can't be acquired within the timeout.
- Consider raising the drive memory limit.
- Add a test: a converter that returns more than the cap gives `conversion_failed` without a full allocation.

**H2. Prod storage I/O goes over the public URL. Unverified, and not needed.**
`preview.go:238-275` (`convert`) GETs the original and PUTs the PDF through presigned URLs for
`https://storage.nexus.zaneng.xyz`. `getent` resolves that to the VPS's own IP 160.187.146.173 (no CDN). So drive
(network `nexus` only) must hairpin: container to host public IP:443, DNAT'd by Docker to the shared Traefik container on
the external `traefik` network, then back to `nexus-minio:9000`. This depends on the host's Docker/iptables
inter-bridge rules and on ufw. Nothing in the repo does this today: every other storage access is server-side through the
document service's internal client. The implementer's live test was native dev, not prod. Also, `docs/deployment.md`
says "Documents never leave the host". The data stays on the box, but it leaves the private network and passes through the
shared Traefik, which other apps use too.
What drive already has: no MinIO credentials, only the `DocumentStorage` gRPC client. The document service holds the
internal client (`MINIO_ENDPOINT`, `document/cmd/main.go:52-73`) and the separate public presign client.
Options, best first:
1. Do server-side I/O in the owner. Add document RPCs that stream an object out and in (or a single `ConvertToPDF`
   RPC whose implementation lives in document, with document joining `previews`). This is a shared proto change, so
   §5 applies: `make proto` and `npm run proto:gen`.
2. Minimal change: keep the presigned URLs but dial `nexus-minio:9000` over plain http from drive's `http.Client`, with
   `req.Host` kept as `storage.nexus.zaneng.xyz`. SigV4 signs the Host header, not the scheme or IP, so MinIO
   accepts it. Use a custom `DialContext` keyed by a `STORAGE_INTERNAL_ADDR` env. Works without any proto change.
3. At minimum, prove the hairpin before deploy: `docker exec nexus-drive-1 wget -qO- https://storage.nexus.zaneng.xyz/minio/health/live`
   (the drive image has wget and ca-certificates). Then fix the deployment.md wording.

## Medium

**M1. First conversion races the nginx 60s timeout.** `deploy/nginx/proxy.inc:12` sets `proxy_read_timeout 60s`.
`PREVIEW_CONVERT_TIMEOUT_SECONDS` defaults to 60, and 4 RPCs run before it. A slow doc gets an nginx 504 (HTML, not the
spec's 200 `unavailable`). The detached conversion still finishes, so the retry works. Set the default timeout to about 45s,
or document the 504.

**M2. Native `text/*` includes `text/html`, and `image/svg+xml` is native too** (`preview_kind.go:98` prefix match,
`nativeExt["svg"]`). The URL is a MinIO object on `storage.nexus.zaneng.xyz`, which is same-site with the app. If the
viewer iframes or navigates to it, uploaded script runs on a sibling subdomain. Download already exposes the same URL,
but a viewer makes "open in place" the default. Map `text/html`, `application/xhtml+xml` and `image/svg+xml` to
`text/plain` (or `unsupported`) server-side. Also pin a viewer contract in the spec: text is fetched and rendered as text,
never iframed.

**M3. Renditions outlive their file.** `drive/previews/<id>/...` is never deleted on permanent delete (documented in
deployment.md as intentional). Purging a sensitive document leaves its full text as a PDF in the bucket indefinitely.
It isn't servable, but it is a retention/erasure gap. Delete the `drive/previews/<id>/` prefix wherever the original
object is deleted, or record the decision in the spec.

**M4. Gotenberg hardening is partial** (`deploy/docker-compose.prod.yml` gotenberg block). It has a memory limit,
`no-new-privileges`, an internal network and no published port, all good. Missing:
- No `cpus` limit. LibreOffice can pin every core of a shared VPS for 70s per document.
- No `pids_limit`.
- The Chromium and webhook routes stay enabled. Only drive can reach them, but add
  `--chromium-disable-routes=true --webhook-disable=true` as defence in depth (they are the URL-fetching features).
  Also note that `previews` contains drive's own REST and gRPC ports. LibreOffice can reach them but has no JWT or HMAC
  identity, which is acceptable.
- Dev compose gotenberg sits on the default network with egress. Dev only, so SSRF is acceptable there.

## Low

- **L1.** `preview.go:254`: a source whose stored `Content-Length` exceeds the cap during fetch becomes `conversion_failed`,
  not `too_large`. When the length is unknown, `LimitReader` silently truncates and LibreOffice gets a cut file. Earlier
  stat checks make both near-impossible.
- **L2.** Each view costs `GetItem` + policy + `GetObjectInfo` (source) + `GetUploadURL` (which calls `ensureBucket`, a
  MinIO round trip) + `GetObjectInfo` (rendition) + `GetDownloadURL`. Build the key locally (the
  `drive/<docId>/<filename>` format is document's), or fetch the upload slot only on a miss.
- **L3.** `preview.go:92` and `gotenberg/client.go`: `&http.Client{}` relies only on the ctx deadline. That is fine, since
  every call carries one.
- **L4.** The "a leaving viewer doesn't cancel the shared conversion" requirement (`context.WithoutCancel`) has no test.
  `preview_domain_test.go` sits in `internal/grpc` but tests the domain only (it reuses the fixtures), which is a misleading
  location.
- **L5.** GetPreview turns any store error into 404 (`preview.go:107`, the same as download). A DB outage reads as
  "file not found".

## Verified OK

- AuthZ is identical to download (`files.go:162-169`): the same GetItem leads to 404 `file not found`, `read` on
  `item.NGACNodeID` and 403 `access denied` with the same words. The folder/pending check comes after authz. There is no
  conversion on deny (test `TestPreview_AuthorizationMatchesDownload` asserts zero converter calls and no keys). The
  existence oracle (404 vs 403) is the same as download's and as the spec's.
- Cache key: `drive/previews/<fileID>/<mtimeNs-size>/preview.pdf` in the file's own workspace bucket. New content gives a
  new key, and a key from a different file is impossible. User upload keys are `drive/<uuid>/<name>`, which cannot
  collide with `drive/previews/`. Tested (`ReplacedContentGetsANewRendition`).
- Singleflight dedups per key, with a re-check after the wait, a detached ctx and a bounded timeout. The concurrency test
  fails without it, which makes it meaningful.
- Layering: REST calls domain only, with no SQL in transport. `check-layering` and `check-ngac` pass. The route is under
  `/api/drive`, which the existing proxy covers. There is no proto change.
- Responses never carry object keys. Gotenberg errors are logged, not returned.

## Unresolved questions
- Prod Docker version and iptables mode: does the published-port hairpin across bridges work on 160.187.146.173? (H2)
- Is deleting renditions on purge in scope, or an accepted retention decision? (M3)

Status: DONE_WITH_CONCERNS
Summary: AuthZ, cache keying, layering and tests are sound. Before deploy, fix the unbounded in-memory PDF buffering
(drive's 128M limit means one large Office file can crash-loop drive) and the dependency on hairpinning through the
public storage URL, which is unverified.
