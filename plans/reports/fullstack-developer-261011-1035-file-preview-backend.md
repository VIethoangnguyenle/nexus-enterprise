# File preview backend (phase 02)

Status: done. Route `GET /api/drive/files/:fileId/preview` (REST only, no proto change) -> `domain.Service.GetPreview`.

- Authz = download's (`read` on item.NGACNodeID, same 404/403 text); folder/pending -> 404 after authz.
- Kinds: native / pdf / unavailable(unsupported|too_large|conversion_failed|conversion_unavailable).
- Office -> Gotenberg `/forms/libreoffice/convert` (adapter `internal/gotenberg`); PDF stored via document service signed PUT at `drive/previews/<id>/<mtime-size>/preview.pdf`; cache hit via GetObjectInfo; singleflight per key on a detached ctx.
- Config: GOTENBERG_URL (dev default http://localhost:3050), PREVIEW_MAX_SOURCE_MB=50, PREVIEW_CONVERT_TIMEOUT_SECONDS=60.
- Infra: gotenberg:8.37.0 in docker-compose.yml (127.0.0.1:3050, 600m) and prod compose (internal `previews` network, 600M, no ports); drive joins it.
- Tests: gotenberg client (7), domain/preview (grpc_test/preview_domain_test.go), route table. drive suite + vet + race green; build-check, check-layering, check-ngac, check-compose-env, check-docs-drift green.
- Live: real .docx -> PDF in 0.58s, cached 16ms; Gotenberg stopped -> conversion_unavailable + WARN; txt native; zip unsupported; unknown 404; no token 401.

## Review round (H1, H2, M1-M4, sheets)
- H1: PDF spooled to temp file (removed after), streamed to PUT; cap PREVIEW_MAX_PDF_MB=80 (-> too_large); semaphore PREVIEW_MAX_CONCURRENT=2 (-> conversion_busy, not remembered); negative cache 10 min for conversion_failed/too_large (outage/busy not cached). Converter iface now `ToPDF(ctx,name,src,dst,max)`.
- H2: PREVIEW_STORAGE_DIAL_ADDR (prod nexus-minio:9000) -> Transport.DialContext override, scheme forced to http, Host stays public. Test asserts Host=public with dial at override. Drive already on `nexus` with MinIO. Docs + post-deploy `docker exec ... wget` checks added; wording corrected.
- M1 timeout 45s. M2 html/xhtml/svg/xml -> native text/plain. M3 permanent delete removes rendition (key recomputed from source info before the original is removed; no document-service change). M4 gotenberg flags: chromium/pdfengines routes, webhook, download-from off, libreoffice-deny-private-ips; prod cpus 1.0 + pids 300 (deploy.limits), dev cpus/pids_limit.
- Sheets: xls/xlsx/ods -> kind "sheet" (original URL, same auth), never converted; csv stays native.
- Added L4 test (viewer leaving doesn't cancel). Gates green: vet, race tests (drive), build-check, check-layering, check-ngac, check-compose-env, docs-drift. Live: docx converted via dial override against dev MinIO (0.88s), xlsx -> sheet, no temp files left.
