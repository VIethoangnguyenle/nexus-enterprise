---
title: Mobile UX overhaul planned and red-teamed
date: 2026-10-19
summary: 12-phase plan from a 390px audit; red team caught a broken NGAC group-management design and a live drive-copy authz bug
---

# Mobile UX overhaul planned and red-teamed

Planned the phone overhaul (bottom bar A, three-scope settings, unified search, gestures, Tài liệu) from a live 390px audit and an approved mockup.

What the red team changed: the first group-management design would have broken Chat. Dropping `write` from a group's Members would also remove reactions, pins, polls, calls, edits and thread replies, because messaging gates all of them on `write`. Nesting a Managers UA inside Members would let removed managers keep access, since messaging only counts direct user edges and migration 061 deletes nested UAs. Workspace owners inherit `manage` on every group, so `manage` cannot mean "group manager". The final design keeps `write`, puts Managers beside Members, enforces top-level posting and @all in messaging, and reuses `channels.created_by` as owner.

Found along the way: drive `CopyItem` checks only read on the source, never write on the destination or same-workspace, and trusts the request's workspace id. It is a live cross-workspace write and will be fixed on its own before the Tài liệu phase.

Lesson: route new endpoints under the owning service's existing prefix (approval is `/api/approval/*`) and check `deploy/nginx/default.conf`, not only the vite proxy.

> Historical work record — not durable authority. Prefer docs/specs/ADRs for current decisions.
