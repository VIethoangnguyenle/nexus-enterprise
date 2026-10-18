# mobile-gestures

## Purpose
Gives the installed phone app the gestures people expect from native apps, without any gesture doing something that
cannot be undone.

## Status
Built (2026-10-19, plan `plans/261018-1742-mobile-ux-overhaul`, phases 09 and 10; design DESIGN.md §6 "Vuốt", mockup
`design/mockups/mobile-shell.html` §7, §9).
- Edge back: `frontend/src/components/patterns/EdgeBackLayer.tsx` (`frontend/src/hooks/useEdgeBack.ts`, `useEdgeBackAction`),
  active in standalone PWA mode on phones; captures DOM snapshot peek underneath with -30% parallax. In-thread edge back peek
  is currently blank because in-thread navigation changes query parameters without changing pathname (`thread peek blank`).
- Axis lock: `frontend/src/hooks/useAxisLock.ts` locks the dominant axis (x or y) after 10px of movement.
- Row swipe: `frontend/src/components/patterns/SwipeRow.tsx` (1:1 finger tracking, 240ms expo settle, open fraction 0.5,
  flick velocity 0.3 px/ms, full swipe fraction 0.6; `SwipeRowGroup` enforces one open row at a time and auto-closes on scroll).
  Chat rows reveal Đã đọc / Chưa đọc (right, full swipe acts with Hoàn tác toast) and Tắt thông báo · Gắn sao · Thêm (left)
  (`frontend/src/components/spaces/chat-row-swipe.tsx`). Approval rows reveal Duyệt… (92px, right) and Từ chối… (left), opening
  confirmation dialogs; full swipe does nothing (`frontend/src/components/approval/ApprovalTable.tsx`).
  Contacts rows reveal Nhắn tin and Gọi (left). Drive rows reveal Chia sẻ · Gắn sao · Thêm (left).
  Notification rows reveal only Đã đọc (right); Xoá and Chưa đọc are not yet built (`Thông báo Xoá/Chưa đọc not yet`).
  Assets rows do not have swipe actions built yet (`Tài sản row swipe not yet`).
- Message gestures: `frontend/src/hooks/useMessageSwipe.ts` (swipe right quotes reply, swipe left opens thread; 64px threshold,
  1.15 scale icon); `frontend/src/components/spaces/ReactionSlider.tsx` (long-press 400ms then slide onto floating toolbar of
  6 reactions 👍 ❤️ 😂 😮 🙏 ✅, 1.5x scale, release over emoji selects, lift without sliding opens sheet);
  `frontend/src/hooks/useDoubleTap.ts` and `frontend/src/hooks/useQuickReaction.ts` (double-tap <= 300ms toggles quick reaction,
  default 👍, customizable in Chat settings).
- Keyboard dismiss drag: `frontend/src/hooks/useKeyboardDismissDrag.ts` (dragging message list down > 48px while keyboard open
  blurs active input).
- Composer grip: `frontend/src/hooks/useComposerGrip.ts` (drag grip above phone composer up > 25% height or 0.5 px/ms flick
  expands to full screen; drag down > 25% collapses; tap toggles).
- Pull to refresh: `frontend/src/hooks/usePullToRefresh.ts` (drag down from top of list past 56px reloads list).
- Drag to close: `frontend/src/hooks/useDragToClose.ts` (drag sheet, Tôi page, or file viewer down past 25% height or 0.5 px/ms
  flick closes).
- Calendar swipe step: `frontend/src/hooks/useSwipeStep.ts` (swipe week strip or month grid horizontally past 48px or 0.5 px/ms
  flick steps period).
- File viewer: `frontend/src/components/files/ImageBody.tsx` (two-finger pinch zoom up to 4x, double-tap zoom 1x/2x, drag pan
  when zoomed). Horizontal swipe navigation checks dx >= 40px on release, but does not track the finger 1:1 during dragging
  (`viewer swipe not 1:1`).

## Requirements

### Requirement: Swiping from the left edge goes back
On every detail screen, settings page and thread in the installed PWA on a phone, a swipe that starts within 20px of the
left edge SHALL follow the finger 1:1 and go back when released past 35% of the width or flicked faster than 0.5 px/ms
(`useEdgeBack`). The layer underneath SHALL show a dimmed still snapshot of the previous screen coming in from -30%
parallax (`EdgeBackLayer`). When opened from a push notification with no previous history, it goes to the module's root
list (`edgeBackRoot`). A swipe starting in the 20px edge zone is claimed for back and is never a row or message swipe.
In-browser tabs and non-standalone modes do not enable edge back and keep the top-bar ← button.

#### Scenario: Edge back from a push notification
- **WHEN** a person opened a conversation from a push notification, so there is no earlier screen, and swipes from the left edge
- **THEN** the app goes to Chat's root list

#### Scenario: Edge zone beats a message swipe
- **WHEN** a person starts a swipe 10px from the left edge on a message in a thread
- **THEN** it is a back swipe, not a reply; the same swipe starting 40px in quotes the message

#### Scenario: Thread edge back has a blank peek (not built)
- **WHEN** a person opens a thread within a conversation and edge-swipes back
- **THEN** the screen goes back to the conversation, but the peek layer underneath is blank because in-thread navigation changes query parameters without triggering router path change snapshots

### Requirement: Gestures never do what cannot be undone
No gesture SHALL delete, edit, approve or reject directly.
- Approval rows in "Chờ tôi" only reveal Duyệt… (92px, accent/success) and Từ chối… (danger); tapping opens a confirmation
  dialog with a comment field; a full swipe across the row does nothing and leaves the buttons revealed (`SwipeRow`).
- Marking read/unread in Chat by a full swipe past 60% width acts at once but SHALL offer a toast with Hoàn tác
  (`setConversationRead`).
- All swipeable rows SHALL enforce the single open row rule via `SwipeRowGroup`: opening a row closes any open row, and
  scrolling the container closes the open row.
- Notifications swipe currently only reveals "Đã đọc"; swipe to delete ("Xoá") and mark unread ("Chưa đọc") are not yet built.
- Assets rows do not yet support swipe gestures.

#### Scenario: Full swipe on a pending request
- **WHEN** a person swipes a request in "Chờ tôi" all the way across
- **THEN** nothing is approved or rejected; the buttons stay revealed

#### Scenario: Full swipe marking read offers Hoàn tác
- **WHEN** a person swipes a conversation row to the right past 60% of the row width
- **THEN** the conversation is marked read at once and a toast appears with an action to Hoàn tác

#### Scenario: Notification row swipe missing delete and unread (not built)
- **WHEN** a person swipes a notification row in the notification panel
- **THEN** only "Đã đọc" is revealed on the right; "Xoá" and "Chưa đọc" are not built

### Requirement: Messages answer swipes, long-press slides and double-taps
In a conversation:
- Swiping a message right past 64px SHALL start a quoted reply, focusing the composer with quote preview (`useMessageSwipe`).
- Swiping a message left past 64px SHALL open its thread (`useMessageSwipe`). Both directions apply identically to one's own messages and others' messages.
- Pressing and holding 400ms then sliding onto the floating reaction row (👍, ❤️, 😂, 😮, 🙏, ✅) SHALL scale the emoji under the finger to 1.5x and rise; releasing over the emoji SHALL react (`ReactionSlider`). Releasing before sliding SHALL open the message action sheet; sliding outside the toolbar SHALL cancel.
- A double-tap within 300ms SHALL toggle the person's quick reaction (👍 by default, configured via `double_tap` in Cài đặt Chat) (`useDoubleTap`, `useQuickReaction`).
- Dragging the message list down past 48px while the virtual keyboard is open SHALL blur the active text input and dismiss the keyboard (`useKeyboardDismissDrag`).
- Dragging the grip handle above the composer up past 25% screen height or flicking > 0.5 px/ms SHALL expand the composer full screen; dragging down past 25% SHALL collapse it; tapping the grip SHALL toggle expansion (`useComposerGrip`).

#### Scenario: Swipe message right quotes reply
- **WHEN** a person swipes a message to the right past 64px
- **THEN** the composer opens with a quoted reply to that message and keyboard focused

#### Scenario: Swipe message left opens thread
- **WHEN** a person swipes a message to the left past 64px
- **THEN** the thread panel or thread view of that message opens

#### Scenario: Long-press slide onto reaction
- **WHEN** a person presses a message for 400ms and slides their finger onto an emoji on the floating reaction toolbar and lifts
- **THEN** the chosen reaction is added to the message

#### Scenario: Double-tap toggles quick reaction
- **WHEN** a person double-taps a message within 300ms
- **THEN** their quick reaction (👍 by default) is toggled on the message

#### Scenario: Dragging message list down closes keyboard
- **WHEN** a person drags the message list down past 48px while typing with the keyboard open
- **THEN** the text field loses focus and the keyboard dismisses

#### Scenario: Composer grip drag expands and collapses
- **WHEN** a person drags the composer grip up past 25% of the screen height
- **THEN** the composer expands to full-screen mode; dragging down past 25% returns it to normal height

### Requirement: Surfaces close by dragging down, lists pull to reload, calendar steps by swipe
- **Pull to reload:** Dragging down from the very top of any scrollable list past 56px (damped by 0.5) SHALL trigger pull-to-refresh and reload data (`usePullToRefresh`).
- **Drag down to close:** Dragging any bottom sheet, the Tôi page, or the file preview modal down past 25% height or flicking > 0.5 px/ms SHALL close the surface (`useDragToClose`).
- **Calendar step:** Swiping the week strip or month grid horizontally past 48px or flicking > 0.5 px/ms SHALL navigate to the next or previous period (`useSwipeStep`).
- **File viewer gestures:** In `FileViewer`, images fit to screen can be pinch-zoomed up to 4x and panned with one finger, or double-tapped to toggle between fit and 2x (`ImageBody`). Swiping horizontally between images at fit zoom navigates to next/previous image on release if distance >= 40px, but does not follow the finger 1:1 during dragging (`viewer swipe not 1:1`).

#### Scenario: Pull to reload a list
- **WHEN** a person pulls down from the top of a list past 56px and releases
- **THEN** the refresh spinner displays and the list re-fetches its data

#### Scenario: Drag sheet down to close
- **WHEN** a person drags a bottom sheet or modal down past 25% of its height
- **THEN** the sheet animates down and closes

#### Scenario: Calendar week swipe
- **WHEN** a person swipes horizontally across the week strip in Lịch past 48px
- **THEN** the calendar steps to the next or previous week

#### Scenario: File viewer swipe is not 1:1 (not built)
- **WHEN** a person swipes horizontally across an image in the file viewer
- **THEN** the image switches upon release if past 40px, but the image does not track the finger 1:1 during the gesture

### Requirement: Every gesture has a button
Each gesture SHALL have an equivalent accessible control:
- Edge back has top-bar ←.
- Row swipes have the ⋯ action menu or long-press bottom sheet.
- Message quote/thread/react have buttons in the long-press action sheet.
- Composer grip drag has tap-to-toggle and screen-reader accessibility.
- Calendar swipe has ‹ and › buttons.
- File viewer swipe has Chevron Left and Chevron Right buttons.
Under reduced motion (`prefers-reduced-motion: reduce`), gestures SHALL NOT follow the finger smoothly; crossing the threshold acts immediately or snaps upon release.

#### Scenario: A gesture without a finger
- **WHEN** a person who cannot swipe wants to reply to a message or go back
- **THEN** the long-press menu has Trả lời / Trích dẫn and the top bar has ←

#### Scenario: Reduced motion disables finger following
- **WHEN** a person with reduced motion enabled performs a swipe gesture
- **THEN** the element does not track the finger continuously during dragging and acts directly once threshold criteria are met
