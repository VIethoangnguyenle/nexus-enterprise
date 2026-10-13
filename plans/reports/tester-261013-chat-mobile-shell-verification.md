# Verification: chat-mobile-shell (phase 07)

Date 2026-10-13, branch claude/inspiring-cray-32y4n1 (phases 02-06 committed). Status: **phase 07 stays pending**. Gates green, but mockup acceptance is not met (see mismatches), so the phase is not marked complete.

## 1. Gates

| Gate | Result |
|---|---|
| `make build-check` | PASS, all services build |
| `make check-layering` | PASS |
| `make check-ngac` | PASS |
| `make test s=messaging` (shared dev DB, port 5433) | FAIL, 238 store/grpc tests skipped (testutil default DSN on 5432 fails auth). Retry with `.env.dev` DSN: 2 FAIL, `notifications` missing column `entity_type` (known drift) |
| `make test s=messaging` (scratch DB `ngac_verify_p07` on same server, from `data/init.sql` + 52 migrations) | PASS, strict mode, 325 runs, all packages ok. DB dropped afterwards, none left |
| `npm test` (vitest) | PASS, 195 files, 2508 tests |
| `npm run lint` | PASS |
| `npm run typecheck:diff` | PASS, no new errors vs baseline |
| `npm run build` | PASS, exit 0. Warnings: 7 "Route file ... does not export a Route" for `*.test.tsx` inside `src/routes/` (move tests out of routes dir); chunk-size warning |

Note: `make dev` ran `db-migrate` against the shared dev DB (idempotent re-apply).

## 2. Stack and deny check

Stack: `make dev` started, stale `.dev-pids` (dead PIDs) removed beforehand. Policy restarted once manually (same env, PID tracked). `make dev-stop` run at end, all app ports confirmed free.

Setup: org workspace `c4e727e1…` (A owner, B member). Group "Đối soát giao dịch": A posts main messages, "Báo cáo đối soát tháng 10 đã có trên thư mục chung." root, B replies, A replies, B follows root. DM A↔B.

Deny evidence (B, through the real drawer workspace switch, not a raw token):

| Read | Before removal (B member) | After A removes B |
|---|---|---|
| `/api/channels/activity` | group present, preview "Báo cáo đối soát tháng 10 …" | absent |
| `/api/messages/search?q=đối soát` | group hits present | absent |
| `/api/threads/followed` (key `messages`) | group present (count 30) | absent (count 29) |
| B sidebar text | "Đối soát giao dịch" shown | not shown |

Removal done through A's app client (`DELETE /api/channels/:id/members/:node`). Group re-added once for a positive baseline, then removed again.

Caveat: first attempt at the before-state read the wrong JSON key and showed false. Corrected. Also a harness issue found along the way: loading `/channels?ws=<org>` directly with B's personal-default token leaves the batch endpoints scoped to the personal tenant (only the DM appears). The real drawer switch fixes it. See Bugs B2.

## 3. Screens (390×844, iPhone 13 emulation, light and dark)

Folder: `plans/reports/chat-mobile-shell-screens/`

- Light and dark, as A: `home`, `home-unread-only`, `direct`, `sections`, `more`, `drawer-org`, `drawer-workspace-list`, `module-approval`, `search-results`, `plus-sheet`, `account-sheet`, `threads-followed`, `thread-screen`. Prefix `light-` or `dark-`.
- Light and dark, as B (personal): `drawer-personal`, `B-home-personal`.
- Desktop: `desktop-1280-channels.png`.
- Mockup per section: `mockup-home`, `mockup-direct`, `mockup-sections`, `mockup-more`, `mockup-drawer`, `mockup-extra`.
- `metrics.json`: scroll widths and bar positions.

Checks:
- Horizontal scroll: none on any screen (scrollWidth = 390).
- Bar overlap: Trang chủ list is short (3 rows); last row bottom 559 vs bar top 724. Clearance not stress-tested with a long list.
- Safe-area: CDP `Emulation.setSafeAreaInsetsOverride` (top 47, bottom 34) applied, effect not measured. Not verified on real iOS.
- Drawer items as spec: org = Chat, Tài liệu, Phê duyệt, Tài sản, Danh bạ, Thông báo, Quản trị, Cài đặt. Personal = Chat, Tài liệu, Danh bạ, Thông báo, Cài đặt. Pass.
- Bar hidden on module and thread screens (barTop null). Pass.
- Desktop 1280: desktop nav present, phone bar and ☰ absent, no overflow. Compared to DESIGN §5 expectations only; main build not compared.

## 4. Mockup mismatches

| Sev | Where | Mismatch |
|---|---|---|
| High | Thêm tab, + sheet | "Duyệt nhóm" row missing in both. DESIGN §5/§6 and mockup section 4 list it. Source has only a comment mentioning it (`NewChatButton.tsx:19`). |
| Medium | ☰ drawer footer | Role line under name missing (mockup: "Lê Thị Hoa · Kế toán trưởng"; app shows name only). DESIGN §5 requires "tên · vai trò". |
| Medium | ☰ drawer, Chat row | No unread pill. DESIGN §5: Chat carries pill. Bar showed unread while drawer showed none (A had unread). |
| Low | ☰ drawer, Phê duyệt | "N chờ bạn" not shown. Not verified (A had no pending items). Check with pending data. |
| Low | Trang chủ rows | Mockup shows presence dots and "@" chip; app shows neither in this data set. Data-dependent, not verified. |
| Low | Dev overlay | ReactQueryDevtools button (palm icon) floats bottom-left over the bar (`main.tsx:41`, `buttonPosition="bottom-left"`). Confirm it is gated to dev only; not in mockup. |

Other visible items match: avatar and bold unread name, "Bạn:" prefix, thread line with curved connector, thread badge on avatar, four icon bar with + beside, Thread button, unread-only toggle.

## 5. Bugs

**B1. Batch read endpoints drop a member's channels when token tenant differs (verify with product).** Kongming's diagnosis, confirmed by the drawer switch. `readableChannelIDs` filters by `TenantID` from the token (`messaging/internal/rest/activity.go:21` and `search.go:26`, `threads.go:121`), with no `TenantMiddleware`. Single-object routes (`GetMessages`) do not scope by tenant, so the same member can read a channel in messages but not in activity. B logs in to its personal default workspace, so org previews are empty until the user switches. The normal UI path switches, so this is not a user-facing break, but the two paths disagree. Repro: sign in as B (default personal), open `/channels?ws=<org>` directly, call `/api/channels/activity`: only DMs returned.

**B2. Direct deep link to a non-default workspace.** `/channels?ws=<org>` shows org channel list while `/api/me` still reports the personal tenant. Hook `useSwitchWorkspace` switches only on workspace entry. Severity medium, deep link or bookmarked URL.

**Policy staleness, ruled out.** Restarting policy did not change results; not a stale in-memory graph.

## 6. Other items done

- Item 5: removed unused CSS utilities `pb-tabbar`, `bottom-tabbar`, `bottom-above-bar`, `pb-bar` from `frontend/src/index.css` (grep: definitions only). `frontend/e2e/personal-workspace.mjs` now opens ☰ drawer (`Mở menu chính`, dialog `Menu chính`) instead of `nav[aria-label="Thêm"]`, `crit` labels updated; `node --check` passes. Not executed against the stack (needs fresh OTP).
- No `MobileTopBar` / `MobileMoreSheet` references remain outside plans.
- No product code changed beyond item 5. Nothing committed or staged.

## 7. Next steps

1. Add "Duyệt nhóm" to Thêm and + sheet (High).
2. Add role line to drawer footer; add Chat unread pill to drawer (Medium).
3. Decide B1: scope batch routes by session workspace the same way as single routes, or document. Add a test that a member sees activity after a tenant switch.
4. B2: switch tenant on `?ws=` entry when token tenant differs.
5. Move `*.test.*` out of `src/routes/`.
6. Confirm dev overlay gating.
7. Re-run messaging against a fresh dev DB after the `notifications` drift is fixed.

## Unresolved questions

- Is `?ws=` deep link meant to switch tenant automatically (B2)?
- Is batch tenant-scoping (B1) an intentional design, or should it follow single-route semantics?
- Safe-area on a real iPhone not checked.
- Main-build comparison for ≥1024 not done (desktop checked against DESIGN only).
- Phê duyệt pending count and mention/presence dots need data to verify.
