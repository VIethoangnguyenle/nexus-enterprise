# Phase 03 · Lịch: one settings place

## Context
Desktop Lịch has `components/calendar/CalendarSettingsDialog.tsx` (Múi giờ, Nhắc trước, Giờ làm việc), opened from the
calendar top bar (`CalendarScreen.tsx`). Phone `modules/CalendarSettings.tsx` has no Giờ làm việc. Both write
`/workspaces/{id}/calendar/settings` (`workStart`, `workEnd` in `lib/settings-defaults.ts`).

## Requirements
1. Cài đặt Lịch gets **Giờ làm việc** on both layouts. Phone: a row "Giờ làm việc" value "08:00–17:30" opening a
   subpage with two choice groups (Bắt đầu, Kết thúc; 30-minute steps). Wide: two time Selects on one row.
   End not after start → the save is refused client-side with an inline message "Giờ kết thúc phải sau giờ bắt đầu."
   (server also refuses, spec scenario "A bad merged value").
2. Wide Lịch page: Tuần bắt đầu vào (Segmented), Múi giờ (Select, same list as the dialog), Giờ làm việc, Lịch đang hiện
   (inline checkboxes with the calendar colour; same store as the calendar's left column), Nhắc trước (Select),
   three notification switches. Chế độ xem mặc định and Thời lượng mặc định stay phone-only.
3. Remove `CalendarSettingsDialog` and its test; the desktop Lịch ⚙ button navigates to `/settings/calendar` keeping `ws`.
4. Each save sends only the changed fields.
5. Update `docs/specs/user-settings/spec.md` scenario "The desktop calendar dialog saves its four fields" → saving the
   working hours from Cài đặt Lịch changes only `workStart`/`workEnd`; check `docs/specs/calendar/spec.md` for the dialog.

## Tests first
- Wide: changing end to 18:00 PUTs `{workEnd:"18:00"}` only; end before start shows the message and sends nothing.
- Phone: Giờ làm việc row shows "08:00–17:30" and its subpage saves.
- Lịch ⚙ (desktop) navigates to `/settings/calendar`; no dialog in the tree.
- Lịch đang hiện checkbox toggles the same visibility the calendar column reads.
