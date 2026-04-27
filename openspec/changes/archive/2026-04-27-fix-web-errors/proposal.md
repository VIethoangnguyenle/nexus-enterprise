# Fix Web Errors — Stabilize NGAC Platform for E2E Usage

## Problem

After deploying the full NGAC platform, multiple runtime errors prevent basic usage:

1. **Drive tables missing** — `drive_items`, `drive_shares`, `drive_quotas` don't exist in running DB (init.sql only runs on fresh volumes)
2. **Drive root not found** — Workspace created before Drive tables existed has no root drive folder
3. **Channel drive 400 errors** — `GET /api/channels/{id}/drive` returns 400 because channel drive auto-creation silently fails
4. **Documents page upload** — Uses legacy `/api/documents/upload` flow that was deprecated in favor of presigned URLs; still partially wired to old endpoint
5. **Workspace guard missing** — Frontend pages (Drive, Documents) use `wsId = workspaces[0]?.id || ''` — when empty string, API calls fire with invalid paths like `/api/workspaces//drive/folders`
6. **No workspace onboarding** — New users see empty state with no way to create a workspace from UI

## Scope

Backend + Frontend fixes only. No UI redesign. No new features. Pure stabilization.

## Who needs this?

Any user logging into the NGAC Platform for the first time and trying to use Channels, Drive, or Documents.
