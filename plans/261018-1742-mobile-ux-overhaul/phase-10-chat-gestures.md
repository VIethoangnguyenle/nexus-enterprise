---
phase: 10
title: "Chat gestures"
status: pending
priority: P2
effort: "1.5d"
dependencies: [9, 4]
---

# Phase 10: Chat gestures

## Overview
Gestures inside a conversation (mockup §9): swipe a message right to quote, left to reply in thread; long-press then
slide to react; double-tap quick reaction; dragging the message list down past a threshold closes the keyboard
(iOS web cannot move the keyboard with the finger, so it blurs the field); drag the composer handle for a
full-screen composer; image viewer swipe between and drag down to close; edge-swipe leaves a thread.

## Requirements
- Functional: thresholds 64px with icon scale 1.15 + accent past threshold; long-press 400ms keeps the finger → slide
  over the reaction row (emoji under the finger scales 1.5), release picks, outside = cancel, lifting before sliding
  opens the existing sheet; double-tap within 300ms toggles `chat.doubleTapReaction` (default 👍, can be off);
  list dragged down > 48px with the keyboard open blurs the composer; composer handle > 25% opens full-screen composer; viewer: horizontal between
  the conversation's images, pinch/double-tap zoom, drag down > 25% closes back into the message.
- Non-functional: delete/edit never gestures; same directions for own and others' messages; reduced motion
  threshold-only; screen reader actions (Trích dẫn, Trả lời trong thread, Thả cảm xúc) on each message.

## Architecture
Reuse `SwipeRow`/`useAxisLock` from phase 09 inside `MessageBlock`; reaction slide extends `MessageActionSheet` /
long-press; keyboard state comes from `useKeyboardOpen` (open/closed on `visualViewport` resize); the setting is
read through `useServerSettings('messaging')` (phase 04).

## Related Code Files
- Modify: `frontend/src/components/spaces/MessageBlock.tsx`, `MessageActionSheet.tsx`, `MessageMenu.tsx`, composer component, file viewer (`file-preview`), thread screen
- Create: `ReactionSlider.tsx`, `useDoubleTap.ts`, tests

## Implementation Steps
1. Tests: quote/thread thresholds; long-press slide selects emoji; lift-early opens sheet; double-tap toggle respects preference; edge zone wins over message swipe; delete not reachable by swipe.
2. Implement; wire preference from phase 04.

## Success Criteria
- [ ] Every row of the §9 table implemented with its button equivalent
- [ ] No regression in existing MessageBlock/MessageActionSheet tests

## Risk Assessment
Gesture conflicts with text selection → swipe starts only outside a text selection; long-press on a link keeps the
existing link menu.

## Rollback
Frontend-only; revert the release.
