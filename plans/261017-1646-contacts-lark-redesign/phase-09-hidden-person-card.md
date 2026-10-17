---
phase: 9
title: "Hidden person card across the app"
status: "completed (not committed)"
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

## Todo / Status (2026-10-18)
- [x] Chat: `MessageBlock` author avatar + name are `PersonLink`s (not on your own messages, not on a grouped line, never the bubble or the hover bar); `ThreadPanel` renders through `MessageBlock`, so replies are covered (tested with a hidden reply author). `MembersPanel` wraps avatar + name + role of each row; the "Tuỳ chọn" menu with "Xoá khỏi nhóm" stays outside the link and gated by `can_remove_members`.
- [x] Mentions: no code change. Test added that a hidden group member stays offered in `MentionDropdown` by the resolver's name (C9). Rendered `@name` inside message HTML is left as text.
- [x] Approvals: approvals know people by NGAC node, so `PersonView.userId` / `ChainItem.userId` are set once the directory or resolver matches the node; `ApprovalDetailPanel` (requester), `ApprovalChain` (approvers, new optional `people` prop) and `AuditList` (actor avatar + name; row keeps actor, action, time) link only then. Roles, departments, unfilled steps and system entries stay plain.
- [x] Documents/drive: `TextsTable` owner (above the row's whole-row link), `DriveDetailPanel` owner, "Người có quyền" user shares (not group/public) and the activity line; `Owner.userId` from `ownerOf`.
- [x] Calendar: `EventPopover` guests (organiser is a guest row) and `GuestsPanel` rows (organiser + guests; remove button apart) via `people.byNodeId`.
- [x] New shared helper `components/primitives/LinkedPerson.tsx`: `PersonLink` when a user id is known, plain children otherwise.
- [x] Tests first (red, right reason), then green: see report. `npm test` 3720 pass, lint clean, typecheck:diff clean, build green.

## Deviations
- Added `primitives/LinkedPerson.tsx` (not listed): approvals, drive and calendar know people by node/optional id.
- Mentions inside rendered message text are not made pressable (HTML from the editor; no safe user id there).
- One control per person: where avatar and name are separate, the avatar link is `secondary` (`tabindex=-1`, `aria-hidden`, still a mouse/touch target); `MembersPanel` wraps both in one link named by `aria-labelledby` (name + role).
- Hidden-member "Nhắn tin trực tiếp" in the MembersPanel menu is existing behaviour, untouched (replying/DM to a hidden person stays possible).

## Review fixes (2026-10-18)
- [x] H1: `GuestsPanel` (the event editor) shows plain guest names; the read-only `EventPopover` rows stay pressable.
- [x] M2: `Popover` keeps a stack of open popovers and marks its panel `data-popover`; only the top one answers Esc, and a press inside a popover opened from it is not "outside" for the ones below. Focus returns to the guest button.
- [x] M3: `PersonLink`/`LinkedPerson` take `name` and `avatarUrl`; every caller passes what it renders, the directory is the fallback.
- [x] M4: `secondary` avatar links (see Deviations).
- [x] M5: `HiddenPersonCard` stops pointer-down, click and context-menu at its root; `PersonLink` stops click and context-menu of the link itself. Keydown is left alone so Esc still reaches the popover. A long press on the author avatar does not open the message sheet (accepted).
- [x] L6: `MembersPanel` link uses `aria-labelledby` (name + role). L8: `ApprovalChain` "(bạn)" is outside the truncated part.
- [x] Q1: `docs/specs/contacts-visibility/spec.md` scenario "A hidden member of a shared group".
- Failed first: `PersonLink.test` (5: name prop, secondary, labelledBy, card events, link events), `MessageBlock.test` (6), `MembersPanel.test` (3), `ApprovalPeople.test` (bạn + one-control), `TextsTable.people.test` (2 context menu), `GuestPeople.test` (nested popover x2, editor plain).
