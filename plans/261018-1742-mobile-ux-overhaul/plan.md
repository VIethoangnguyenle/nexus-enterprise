---
title: "Mobile UX: thanh đáy kiểu Lark, cài đặt ba cấp, tìm và vuốt"
description: "Phone shell with a Lark-content / Google-Chat-form bottom bar, one search for every task, settings in three scopes (Tôi, module, conversation), group management rights, swipe gestures (app-wide and in Chat), and a phone-first Tài liệu."
status: in-progress
priority: P1
effort: "~26d"
branch: feat/mobile-ux-overhaul
base: main (36edb19)
tags: [mobile, pwa, frontend, settings, search, gestures, drive, ngac, design-first]
created: 2026-10-19
blockedBy: [fix-drive-copy-cross-workspace]
blocks: []
related: [261010-2258-pwa-mobile-push (navigation part superseded by this plan), 261010-2258-personal-workspace (its personal → organization conversion is removed by phase 05)]
brainstorm: ../reports/brainstorm-261019-0004-mobile-ux-settings.md
audit: ../reports/mobile-ux-audit-261019/
mockup: design/mockups/mobile-shell.html
redteam: [reports/redteam-security-adversary.md, reports/redteam-failure-modes.md, reports/redteam-assumptions.md, reports/redteam-scope-contracts.md]
migrations: {phase-04: 068_user_settings.sql, phase-07: 069_channel_management.sql, phase-11: 070_drive_stars.sql}
specs:
  new: [mobile-shell, user-settings, conversation-settings, chat-group-management, chat-mention-all, unified-search, mobile-gestures]
  modified: [tenant-ngac-init, contacts-visibility, chat-sections, search-suggestions, chat-threads, drive-tree-navigation, drive-permission-engine, file-preview, notifications, workspace-admin-authorization, resource-pep-coverage, tenant-identity, contacts-documents-settings-screens, pwa-install]
---

# Mobile UX: thanh đáy kiểu Lark, cài đặt ba cấp, tìm và vuốt

Inputs (authoritative, every bullet binding): brainstorm `../reports/brainstorm-261019-0004-mobile-ux-settings.md`
(decisions 1–15), mockup `design/mockups/mobile-shell.html` §1–§10 (bottom bar **A**, Zane 2026-10-19), audit
`../reports/mobile-ux-audit-261019/`, red-team reports in `reports/` (15 findings accepted by Zane 2026-10-19).
Below 1024px only; desktop layout does not change except where a shared component (search, conversation settings
panel, Quản lý nhóm) gains content.

## Contract

- **Outcome:** on an installed iPhone PWA every module is one tap away on a floating bottom bar (4 slots the person
  picks + Thêm); Chat keeps Mục and gets chip filters, scroll-compact bar, quick jump and message swipes; every
  module has one search and "Cài đặt <module>"; Tôi holds personal settings in two layers (Tài khoản, Trong
  workspace này); groups and DMs have full settings, with group management rights enforced server-side; one search
  finds people, conversations, messages, documents, requests, events, assets and actions; swipe gestures work
  app-wide; Tài liệu is list-first with Truy cập nhanh, star, multi-select and background upload. Personal and
  organization workspaces each show only what they have.
- **Constraints:** design-first (phase 01 before UI code); `ak:cook --tdd`; every new decision path tested on its
  deny case (policy test vectors in `backend/services/policy/internal/ngac/` for graph changes); NGAC strings only
  from `backend/ngac`; every graph write through EPP invalidation; every new REST route in `frontend/vite.config.js`
  **and** `deploy/nginx/default.conf`, using the owning service's existing prefix; schema changes in the reserved
  migration files, idempotent (`make db-migrate` re-runs every file), applied and observed against a running DB
  after a backup, never edited once applied; no raw identifier on screen; proto change ⇒ `make proto` **and**
  `npm run proto:gen`; each backend phase has a rollback section.
- **Non-goals:** native app; 768–1023 rail; desktop settings redesign; approval delegation; document scanning; Web
  Share Target; read receipts setting and its lock (no "seen by" screen exists — later round); converting workspace
  kinds in either direction.
- **Acceptance:** phase success criteria all met; at 390px (light + dark) e2e screenshots for each module, Tôi,
  each module settings page, group + DM settings, Quản lý nhóm, search (empty, Tất cả, scoped), each gesture state,
  for **both** workspace kinds; deny-case tests for every group-management, lock, search, star and quick-access
  decision; real iPhone PWA pass (phase 12) with touch gestures verified by a person.

## Prerequisite (separate, before phase 11)

`fix-drive-copy-cross-workspace` — production authorization bug found by the red team: `CopyItem`
(`backend/services/drive/internal/domain/items.go:223-273`) checks only read on the source, never write on the
destination folder nor that it is in the same workspace, and takes the copy's workspace from the request. Fixed on
its own branch via `ak:debug` → `ak:fix` with a full deny matrix, deployed before phase 11. Phase 11 reuses the fixed
route.

## Phases

| # | Phase | Effort | Depends on | Status |
|---|-------|--------|------------|--------|
| 01 | [Design and specs](./phase-01-design-and-specs.md) | 2d | — | Done (12a8bec) |
| 02 | [Bottom bar and shell](./phase-02-bottom-bar-shell.md) | 2.5d | 01, 04 | Done (built, awaiting device check) |
| 03 | [Module headers and one search entry](./phase-03-module-headers.md) | 1d | 02 | Done (built, awaiting device check) |
| 04 | [Settings backend](./phase-04-settings-backend.md) | 4d | 01 | Done (built, awaiting device check) |
| 05 | [Tôi, workspace and module settings](./phase-05-personal-and-module-settings.md) | 3.5d | 02, 04, 08 | Done (built, awaiting device check) |
| 06 | [Conversation settings](./phase-06-conversation-settings.md) | 1.5d | 02 | Done (built, awaiting device check) |
| 07 | [Group management](./phase-07-group-management-ngac.md) | 3.5d | 06 | Done (built, awaiting device check) |
| 08 | [Unified search](./phase-08-unified-search.md) | 2d | 03 | Done (built, awaiting device check) |
| 09 | [Gestures foundation](./phase-09-gestures-foundation.md) | 2d | 03 | Done (built, awaiting device check) |
| 10 | [Chat gestures](./phase-10-chat-gestures.md) | 1.5d | 09, 04 | Done (built, awaiting device check) |
| 11 | [Tài liệu on the phone](./phase-11-drive-mobile.md) | 2.5d | 08, 09, prerequisite | Done (built, awaiting device check) |
| 12 | [Verify on a real iPhone](./phase-12-verify-iphone.md) | 0.5d | all | Pending |
| 13 | [Danh bạ on the phone](./phase-13-contacts-mobile.md) | 2d | 08, 09 | Done (built, awaiting device check) |

Order: 01 → 04 → 02 → 03 → {08 → 05}, {06 → 07}, {09 → 10, 09 + 08 → 11, 09 + 08 → 13} → 12. Phases that edit `PhoneSearchBar`,
`MobileNav`/`ModuleBar` or `nav-destinations.ts` (02, 03, 05, 08, 09) run one after another, never in parallel.

## Key decisions carried from the brainstorm and red team

1. Bar A: floating pill holding modules; defaults Chat · Lịch · Tài liệu · Phê duyệt (personal: Danh bạ); per
   person, per workspace; ☰ drawer removed (workspace switching moves to a new sheet).
2. Chat: chips Tất cả · Mục · Chưa đọc · Được nhắc · Trực tiếp · Nhóm; Mục keeps existing section behaviour; desktop
   routes `/channels/mentions`, `/channels/starred` stay.
3. Settings: Tài khoản (per person) + Trong workspace này + Cài đặt workspace (one page, rows by permission, holds Quản
   trị on phones) + module pages + conversation pages; each option lives in
   the service that enforces it; org lock this round: **"Ai được tạo nhóm" only**, enforced by removing the `create_channel` right (Members and custom roles; Owners keep theirs), not by a prohibition.
4. Workspace kind fixed at creation: convert-to-organization UI and route removed.
5. Group management: owner = `channels.created_by`; managers = a per-group UA **beside** Members; members keep
   `write`; "only managers post" and @Tất cả are enforced in messaging on send, reply and edit; manager authority is
   membership in the Managers UA, never `manage` (workspace owners inherit `manage`).
6. When posting is restricted members still reply in threads (Zane 2026-10-19).
7. Gestures never perform an irreversible action; approval swipes only reveal buttons.
8. "Ghim" quick action in conversation settings is Gắn sao (star); no pin-to-top feature.
9. Quiet hours block push except when the person is mentioned by name, `@Tất cả` included.
10. `@Tất cả` in the workspace-wide channel: workspace administrators only; not available in DMs.
11. Workspace owners keep their existing `manage` moderation in groups (rename, picture, remove member, delete message,
    close poll, edit task) but gain no group-management power; group owner and managers pass the same checks via the
    Managers UA.

## Open questions

None.

## Red Team Review

### Session — 2026-10-19
**Findings:** 41 raw → 15 after dedup (15 accepted, 0 rejected). **Severity:** 4 Critical, 9 High, 2 Medium.

| # | Finding | Severity | Disposition | Applied To |
|---|---------|----------|-------------|------------|
| 1 | Managers UA nested in Members: removed/left managers keep access or cannot be removed; 061 reconcile deletes it | Critical | Accept | Phase 07 |
| 2 | Dropping `write` from Members breaks reactions, pins, polls, calls, edits and thread replies | Critical | Accept | Phase 07 |
| 3 | Workspace owners inherit `manage` on every group → self-promotion into private groups, @all bypass | Critical | Accept | Phase 07, 01 |
| 4 | Owner duplicates `created_by`; creator bypass after transfer; backfill re-admits leavers | High | Accept | Phase 07, 06 |
| 5 | Existing drive copy writes across workspaces (production bug) | Critical | Accept | Prerequisite, Phase 11 |
| 6 | @Tất cả does not exist; edits can add mentions | High | Accept | Phase 07, 01 |
| 7 | "Ai được tạo nhóm" lock bypassed by custom roles; built-in grants not editable | High | Accept (Zane 2026-10-19: implemented by removing the right, not a prohibition) | Phase 04 |
| 8 | Read-receipt lock has nothing to enforce; enforcing on write breaks unread counts | High | Accept (dropped this round, Zane) | Phase 04, 05, plan |
| 9 | Phase 04 wrong service ownership, invalid PK, `usePreferences` name clash, no membership check, notify keys unwired | High | Accept | Phase 04 |
| 10 | New routes on wrong prefixes; production nginx not updated | High | Accept | Phase 08, 11, plan |
| 11 | Star / quick access mix workspaces; mention reason leaks chat activity | High | Accept | Phase 11 |
| 12 | Migration numbers collide; not idempotent; no rollback | High | Accept | Plan, 04, 07, 11 |
| 13 | Redirecting desktop Chat panes; edge-back with no history after a push tap; existing back hooks ignored | High | Accept | Phase 02, 09 |
| 14 | Dependency/spec/file-list gaps; WorkspaceSwitcherSheet missing; iOS keyboard drag impossible; 30s client permission cache | Medium | Accept | Plan, 01, 02, 05, 06, 07, 10 |
| 15 | Mockup rows without backend (pause, language, sessions, drafts, reasons); upload queue lost on reload | Medium | Accept | Phase 04, 05, 11 |

### Whole-Plan Consistency Sweep
Done after applying: removed every mention of `owner_id`, a prohibition-based lock, Managers-inside-Members, dropping `write`, `manage` as
manager test, `replies_open`, `/api/workspaces/:id/approvals|assets|drive/...` routes, new copy route,
`usePreferences(scope)`, read-receipt lock, `0NN_` migration placeholders, Chat-route redirects and keyboard-drag
dismissal; dependencies table matches phase frontmatter. Zero unresolved contradictions.

## Deferred to a later phase (2026-10-19, Zane: ship the bug fixes first)
- Thông báo: Xoá and Chưa đọc (backend routes + row swipe).
- Tài sản: row swipe (Giao/Thu hồi when allowed, Thêm); wire the "Sắp xếp" row to the list's new `sort` param.
- Approval `default_tab` accepting the screen's tabs (pending|mine|history|department); link previews; drive and asset
  notification producers for the hidden switches.
- Trashed group drives after disband still count against quota (operator purge only); requester told "approved" on a
  partial approval; thread edge-back peek is blank; image viewer swipe is not 1:1.
- Danh bạ: search opens its own screen, "Mời vào cuộc họp" does not add the guest, "Mời thành viên" opens Cài đặt
  workspace.
- One-off after deploy: run the group managers backfill (docs/deployment.md).
