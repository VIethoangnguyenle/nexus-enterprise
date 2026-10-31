# Desktop settings parity

Status: in progress · Branch `feat/web-settings-parity` (from `origin/main` 03ed380) · Worktree `.claude/worktrees/web-settings`

## Outcome
On a screen ≥ 1024px, Cài đặt offers every option the phone offers (DESIGN.md §6 "Cài đặt trên máy tính", mockup
`design/mockups/settings-desktop.html`, approved 2026-11-01), reading and writing the same stores, at the same
addresses (`/settings/<section>`). Lịch has one settings place.

## Constraints
- Design source: the approved mockup and DESIGN.md §6. Do not design in code.
- One component per section for both layouts. Phone behaviour and phone tests stay as they are.
- Same hooks and endpoints as the phone; no backend change, no new endpoint, no schema change.
- Every PUT sends only the changed field (spec `user-settings` "Every settings PUT is a merge").
- No raw identifier on screen; Vietnamese copy as in the mockup and the phone pages.
- TDD: a failing test first for each behaviour. `cd frontend && npm test`, `npm run lint`, `npm run build` green.

## Non-goals
- New options neither layout has (Xem trước nội dung, Màu theo người).
- Changing the Quản trị screens themselves; rows only link to them.
- Phone redesign (only additions: Giờ làm việc row in Cài đặt Lịch).

## Capability specs touched
- `docs/specs/user-settings/spec.md` (desktop entry, calendar dialog scenario replaced).
- `docs/specs/contacts-documents-settings-screens/spec.md` (Cài đặt tabs → section list).
- `docs/specs/calendar/spec.md` if it names the settings dialog.

## Phases
| # | Phase | Depends |
|---|-------|---------|
| 01 | [Shell, routing, wide rendering of the settings pieces](phase-01-shell-and-routing.md) | — |
| 02 | [Adaptive controls and module pages](phase-02-adaptive-controls-and-modules.md) | 01 |
| 03 | [Lịch: one settings place](phase-03-calendar-one-place.md) | 02 |
| 04 | [Account group: account, org profile, notifications, appearance, sessions](phase-04-account-group.md) | 02 |
| 05 | [Cài đặt workspace on desktop](phase-05-workspace-page.md) | 02 |
| 06 | [Entry points, retire tabs, specs, verification](phase-06-entry-points-and-verification.md) | 03–05 |

## Acceptance criteria
1. At 1440px, `/settings` shows the section list (two groups, version line) and Tài khoản và hồ sơ; each list item is a
   link to `/settings/<section>`; reload/Back keep the section; `?tab=ho-so|workspace|giao-dien|thong-bao` open
   account|workspace|appearance|notifications.
2. Personal workspace: list has no Hồ sơ trong tổ chức, Phê duyệt, Tài sản; their addresses land on Tài khoản và hồ sơ.
3. Every row of the inventory (`plans/reports/settings-gap-inventory.md`) marked MISSING is present on desktop, except
   the touch-only rows listed in DESIGN.md, which are absent on desktop and unchanged on phone.
4. Choices are inline Select (≥ 4) or Segmented (2–3) and save on change; a failed save restores the value and toasts
   "Chưa lưu được…". Switches save on press.
5. Lịch: Giờ làm việc in Cài đặt Lịch on both layouts; no CalendarSettingsDialog; the Lịch ⚙ opens `/settings/calendar`.
6. Phone: every existing phone settings test passes unchanged (except tests of the removed dialog).
7. Visual check at 1440 light/dark against the mockup screens 1–6; screenshots in `plans/reports/`.
