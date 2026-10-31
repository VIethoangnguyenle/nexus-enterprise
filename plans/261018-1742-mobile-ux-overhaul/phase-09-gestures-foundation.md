---
phase: 9
title: "Gestures foundation"
status: completed
priority: P2
effort: "2d"
dependencies: [3]
---

# Phase 09: Gestures foundation

## Overview
App-made edge-swipe back (iOS PWA has none), row swipe actions, axis locking, Lịch week/month swipe, pull to refresh
everywhere, drag-down to close for sheets and pages (mockup §7).

## Requirements
- Functional: edge zone 20px left → back with the previous screen peeking; release > 35% or velocity > 0.5 px/ms;
  row swipe left reveals 2–3 actions (Chat: Tắt thông báo · Gắn sao · Thêm; Tài liệu: Chia sẻ · Gắn sao · Thêm; Tài sản;
  Thông báo: Xoá), right on Chat/Thông báo = Đã đọc/Chưa đọc (full swipe > 60% acts, Hoàn tác toast); Phê duyệt rows
  reveal Duyệt…/Từ chối… only (full swipe does nothing); one open row at a time; Lịch dải tuần/lưới tháng swipe;
  pull to refresh on every list; sheets/Tôi/viewer drag down to close.
- Non-functional: axis lock after 10px; 1:1 tracking, release 240ms expo; reduced motion = threshold-only, no
  tracking; every gesture has a button equivalent; no `navigator.vibrate` reliance (iOS lacks it).

## Architecture
Built new on `motion/react` pointer/pan handlers (the drawer's single `drag="x"` is not a reusable layer and is
deleted in phase 02): `useEdgeBack` drives the existing back logic (`useDeviceBack` / `useShortcutScreen`) instead of
a second history model; when there is no in-app history (app opened from a push notification) back goes to the
module's root list; previous screen peeks via a snapshot layer. `SwipeRow` (reveal actions, coexists with
`useLongPress`), `useAxisLock`; existing `usePullToRefresh` generalized.

## Related Code Files
- Create: `frontend/src/hooks/useEdgeBack.ts`, `useAxisLock.ts`, `frontend/src/components/patterns/SwipeRow.tsx`, `EdgeBackLayer.tsx`, tests
- Modify: `frontend/src/hooks/usePullToRefresh.ts`, `useLongPress.ts`, chat list rows (`HomeView`), notification rows, approval rows, calendar week strip

## Implementation Steps
1. Unit tests with synthetic pointer events: axis lock, thresholds, one open row, approval full swipe no-op, reduced motion.
2. Build hooks/components; apply to listed screens.
3. Verify scroll performance (no layout thrash) on 300-row list in a perf test.

## Success Criteria
- [ ] Edge back works on every detail screen; never starts a row swipe
- [ ] Approval cannot be approved/rejected by a swipe alone (test)
- [ ] vitest + lint + build green

## Risk Assessment
iOS Safari reserves the very left edge in browser mode → in standalone PWA it does not; browser mode falls back to
the ← button (detect `display-mode: standalone`).

## Rollback
Frontend-only; revert the release.
