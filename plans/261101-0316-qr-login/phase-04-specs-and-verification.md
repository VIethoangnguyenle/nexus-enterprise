# Phase 04 — Specs, design doc, verification

## Docs

- `docs/specs/tenant-auth-flow/spec.md` — add "Sign-in by QR": states, TTL, single use, poll
  binding cookie, tenant = phone's current tenant, membership re-check, error codes.
- `docs/specs/sign-in-screens/spec.md` — corner switch, QR states, phone scanner, confirm and
  error screens; link `design/mockups/qr-login.html`.
- `DESIGN.md` — one entry pointing to the QR mockup and the two documented colour exemptions
  (QR white quiet zone, dark camera surface).

## Verification

1. `make build-check`, `make check-layering`, `make check-ngac`, `make test s=auth`.
2. `cd frontend && npm test && npm run lint && npm run build`.
3. End-to-end with two Playwright contexts (desktop + phone-sized, phone signed in): create →
   scan → approve → desktop lands signed in with the phone's tenant; plus decline, expiry, and
   "third context with the token but no poll cookie never gets a session".
4. `ak:code-review` on the full diff; fix findings.

## Rollout

No migration, no new service, no new proxy entry. Production nginx already routes `/api/auth`
and serves the SPA fallback for `/qr/…` (verify in phase 01).
