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
phase 06). Clicking them shows the small card, never details or actions. Spec: `contacts-visibility` (UI scenarios).

## Requirements
- Wrap author/actor/owner name+avatar with `PersonLink` (phase 06) in:
  - Chat: `components/spaces/MessageBlock.tsx` (author), `components/spaces/MembersPanel.tsx` (channel members),
    `components/chat/ThreadPanel.tsx` (thread author, if it renders its own header).
  - Approvals: `components/approval/ApprovalDetailPanel.tsx` (requester, approvers), `components/approval/AuditList.tsx`
    (actor).
  - Documents: `components/documents/TextsTable.tsx` (owner), `components/drive/DriveDetailPanel.tsx` (owner/sharer).
- Visible person → open Danh bạ profile (plan Q7 default); hidden → `HiddenPersonCard`.
- Replying in an existing conversation with a hidden person stays possible (no change to composer/DM).
- Mentions: candidates in a channel come from channel members; if `MentionDropdown` builds from `dir.list`, hidden members
  of the same group are not offered — verify and, if so, source candidates from channel members + resolver (plan Q-open:
  keep consistent with "trả lời vẫn được").
- No raw IDs; card has no Nhắn tin/Gọi.

## Architecture
Callsite-only edits; `PersonLink` does visibility via `usePeople().isVisible`. No new endpoints.

## Related Code Files
- Modify: `frontend/src/components/spaces/MessageBlock.tsx`, `spaces/MembersPanel.tsx`, `chat/ThreadPanel.tsx` (verify),
  `chat/MentionDropdown.tsx` (only if verification says so), `approval/ApprovalDetailPanel.tsx`, `approval/AuditList.tsx`,
  `documents/TextsTable.tsx`, `drive/DriveDetailPanel.tsx`, and their existing tests

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
