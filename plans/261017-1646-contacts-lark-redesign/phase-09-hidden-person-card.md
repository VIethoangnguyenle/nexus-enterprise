---
phase: 9
title: "Hidden person card across the app"
status: pending
priority: P2
effort: "5h"
dependencies: [6]
---

# Phase 9: Hidden person card across the app

## Overview
People outside the caller's visibility keep their name + avatar wherever the caller already works with them (resolver,
phase 06). Clicking them shows the small card, never details or actions. Specs: `contacts-visibility` (UI scenarios),
`calendar` (guest rows).

## Requirements
- Wrap author/actor/owner name+avatar with `PersonLink` (phase 06) in:
  - Chat: `components/spaces/MessageBlock.tsx` (author), `components/spaces/MembersPanel.tsx` (channel members),
    `components/chat/ThreadPanel.tsx` (thread author, if it renders its own header).
  - Approvals: `components/approval/ApprovalDetailPanel.tsx` (requester, approvers), `components/approval/AuditList.tsx`
    (actor).
  - Documents: `components/documents/TextsTable.tsx` (owner), `components/drive/DriveDetailPanel.tsx` (owner/sharer).
  - Calendar: `components/calendar/EventPopover.tsx` `Guest` (:51-60, name/avatar come from the calendar payload
    `a.person.*`, so names already render; wrap for the click) and `GuestsPanel.tsx` guest rows. Pickers there read
    `usePeople().list` (:74) and so offer only visible people after phase 06 (plan Q3).
- Visible person → open Danh bạ profile (C7, Zane 2026-10-17); hidden → `HiddenPersonCard`.
- Replying in an existing conversation with a hidden person stays possible (no change to composer/DM).
- Mentions: verified on main — `MentionDropdown` already lists channel members by display name
  (`components/chat/MentionDropdown.tsx:13-21`), so hidden members of a shared group stay offered (C9). Only check that
  their names resolve through the resolver (test), no code change expected.
- `MembersPanel` keeps "Xoá khỏi nhóm" gated by `can_remove_members` (security phase 08 review fix); `PersonLink` wraps only
  avatar + name, never that action.
- No raw IDs; card has no Nhắn tin/Gọi.

## Architecture
Callsite-only edits; `PersonLink` does visibility via `usePeople().isVisible`. No new endpoints.

## Related Code Files
- Modify: `frontend/src/components/spaces/MessageBlock.tsx`, `spaces/MembersPanel.tsx`, `chat/ThreadPanel.tsx` (verify),
  `approval/ApprovalDetailPanel.tsx`, `approval/AuditList.tsx`, `documents/TextsTable.tsx`, `drive/DriveDetailPanel.tsx`,
  `calendar/EventPopover.tsx`, `calendar/GuestsPanel.tsx`, and their existing tests

## Implementation Steps (TDD)
1. Tests per callsite: hidden author shows real name (resolver mock) and click opens the card with the exact sentence and
   no buttons; visible author click navigates to the profile; no text matches a UUID regex; audit row still shows actor +
   action + timestamp (CLAUDE.md §5 identifiers rule).
2. Implement; `npm test`, `npm run lint`, `npm run build`.

## Success Criteria
- Mockup §1c message example reproduced: hidden "Nguyễn Hữu Thành" message shows name/avatar; click → small card.

## Risk Assessment
- Click target conflicts with existing message hover actions (Med×Low): PersonLink only on avatar + name, not the bubble.
- Rollback: revert callsites; resolver keeps names working.
