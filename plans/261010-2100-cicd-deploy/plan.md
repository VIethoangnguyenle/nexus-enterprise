---
title: "CI/CD deploy to the VPS"
description: "Build images on GitHub Actions, push to GHCR, and deploy to 160.187.146.173 behind the shared Traefik at nexus.zaneng.xyz after a manual approval."
status: done
priority: P1
branch: refactor/project-wide
created: 2026-10-10
---

# CI/CD deploy to the VPS

## Decisions (user, 2026-10-10)
1. Server: root@160.187.146.173 (Ubuntu 24.04, 4 vCPU, 8 GB, Docker 29, Compose v5).
2. Trigger: push to `main` → CI green → images → **manual approval** in the GitHub `production`
   environment → deploy.
3. Images: produced by GitHub Actions, pushed to **private GHCR**; the server only pulls.
4. Domain: **https://nexus.zaneng.xyz** (needs DNS A record `nexus` → 160.187.146.173).

## Server facts (read-only inspection, 2026-10-10)
- A shared Traefik v3.7 already owns :80/:443 (`/opt/traefik`): entrypoint `websecure`,
  cert resolver `le` (Let's Encrypt HTTP challenge), docker provider on the external network
  `traefik`, `exposedByDefault: false`. Nexus must join that network with labels — never run a
  second Traefik or publish ports (Docker-published ports bypass ufw).
- House convention (`/opt/README-deploy.md`): no image builds on the VPS, each app under
  `/opt/<app>`, secrets in a chmod-600 `.env`. Odysseus (zaneng.xyz) is stopped; OpenClaw is
  loopback-only.

## Outcome
A merge to `main` deploys a tested build to https://nexus.zaneng.xyz after one click, with a
database backup before every migration and an automatic rollback to the previous images if the
health check fails.

## Constraints
- Never publish container ports; route only through the shared Traefik.
- No long-lived registry token on the server: the deploy job logs the server into GHCR with
  its short-lived `GITHUB_TOKEN`, pulls, and logs out.
- SSH host key pinned (`DEPLOY_KNOWN_HOSTS`); a dedicated deploy key, not a personal one.
- Production must not run the fixed OTP test code.
- Migrations run once each (ledger), after a `pg_dump` backup.

## Non-goals
- Multi-server, Kubernetes, blue/green.
- Moving Odysseus or OpenClaw.

## Acceptance criteria
- [x] `deploy.yml` produces and pushes every image tagged with the commit SHA.
- [x] The deploy job waits for approval, deploys, runs migrations after a backup, and passes a
      health check on https://nexus.zaneng.xyz.
- [ ] A failing health check rolls back to the previous SHA.
- [x] Nothing in the Nexus stack publishes a host port; ufw unchanged.

## Steps needing the user (outward-facing)
- DNS record; GitHub `production` environment with required reviewer; repo secrets
  (`DEPLOY_SSH_KEY`, `DEPLOY_KNOWN_HOSTS`); one-time server bootstrap (create `/opt/nexus`,
  `.env`, install the deploy public key); Google OAuth client / email sender for real sign-in.

## Result (2026-10-10)
First deploy of `d9817e4` approved and green: https://nexus.zaneng.xyz serves 200 with a Let's
Encrypt certificate, http redirects to https, the 14 Nexus containers are healthy with no
published ports, and `storage.nexus.zaneng.xyz` serves MinIO. The rollback path is in place but
has not fired yet (the first deploy has no previous release to roll back to).
