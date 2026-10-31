# Phase 02 · Adaptive controls and module pages

## Context
Module pages (`components/settings/modules/{Chat,Calendar,Drive,Approval,Assets}Settings.tsx`) open a `ChoicePage`
subpage for each choice through local `page` state, and gate compact rows with `useBottomBarLayout()`.

## Requirements
1. `SettingsChoice` row (new, in `SettingsGroup.tsx` or its own file): props `label, hint?, icon?, value, choices,
   onPick, onOpen, locked?`.
   - phone: identical to today's `SettingsRow` with `value={choiceLabel(...)}` and `onPress={onOpen}` (the page keeps
     its existing `ChoicePage` subpage).
   - wide: the row with an inline control on the right: `Segmented` (primitives) when 2–3 choices, else `Select`
     (primitives/Select.tsx; extend it only if needed for an accessible label). Picking calls `onPick` at once.
2. Touch-only gating: rows listed in DESIGN.md §6 "Cài đặt trên máy tính" render only on phone. Use the existing
   `useBottomBarLayout()` gate where present; add it for Cảm xúc khi chạm đúp (Chat) and Tải ảnh xem trước (Tài liệu).
3. Apply `SettingsChoice` in Chat (Mức mặc định cho nhóm mới), Drive (Xem dạng, Sắp xếp), Assets (Tab mở đầu),
   Approval (keep Sắp xếp phone-only). Lịch is phase 03.
4. Pages set `lead` (phase 01) = "Trong <workspace name>".
5. Save failures keep today's behaviour (value restored, toast "Chưa lưu được…"), now also from the inline control.

## Files
Modify: `SettingsGroup.tsx`, `modules/ChatSettings.tsx`, `modules/DriveSettings.tsx`, `modules/AssetsSettings.tsx`,
`modules/ApprovalSettings.tsx`, possibly `primitives/Select.tsx`. Tests: extend the module settings tests
(`*Screen.settings.test.tsx`, `PhoneSettings.test.tsx`) with wide-layout cases rendered inside `SettingsShell`.

## Tests first
- Wide Chat: four kind switches, default level as a Select that PUTs only `{default_level}`; no Enter để gửi, Mở Chat ở,
  Sắp xếp, Cảm xúc khi chạm đúp; Quản lý mục links to `/channels?tab=sections`; org locked row when locked.
- Wide Drive: Xem dạng / Sắp xếp inline; Thư mục trước tệp and Được chia sẻ tài liệu switches; no Tải ảnh xem trước.
- Wide Assets: Tab mở đầu inline. Wide Approval: two switches, no Sắp xếp.
- A failed PUT restores the shown value.
- Phone tests unchanged and green.
