# mobile-gestures

## Purpose
Gives the installed phone app the gestures people expect from native apps, without any gesture doing something that
cannot be undone.

## Status
Specified ahead of code (2026-10-19, plan `plans/261018-1742-mobile-ux-overhaul`, phases 09–10; design DESIGN.md §6
"Vuốt").

## Requirements

### Requirement: Swiping from the left edge goes back
On every detail screen, settings page and thread, a swipe that starts within 20px of the left edge SHALL go back when
released past 35% of the width or fast; with nothing to go back to (opened from a push notification) it goes to the
module's root list. A swipe that starts in the edge zone is never a row or message swipe.

#### Scenario: Edge back from a push notification
- **WHEN** a person opened a conversation from a push notification, so there is no earlier screen, and swipes from the left edge
- **THEN** the app goes to Chat's root list

#### Scenario: Edge zone beats a message swipe
- **WHEN** a person starts a swipe 10px from the left edge on a message in a thread
- **THEN** it is a back swipe, not a reply; the same swipe starting 40px in quotes the message

### Requirement: Gestures never do what cannot be undone
No gesture SHALL delete, edit, approve or reject. Approval rows only reveal Duyệt… / Từ chối…, which open a confirmation;
a full swipe on them does nothing. Marking read/unread by a full swipe offers Hoàn tác.

#### Scenario: Full swipe on a pending request
- **WHEN** a person swipes a request in "Chờ tôi" all the way across
- **THEN** nothing is approved or rejected; the buttons stay revealed

### Requirement: Messages answer swipes, long-press slides and double-taps
In a conversation, swiping a message right SHALL start a quoted reply, swiping left SHALL open its thread; long-press then
sliding onto the reaction row and releasing SHALL react; a double-tap SHALL toggle the person's quick reaction (👍 by
default, can be turned off). Dragging the message list down past 48px with the keyboard open SHALL close the keyboard.

### Requirement: Every gesture has a button
Each gesture SHALL have an equivalent control (long-press menu, ⋯, ← or a visible button) and a screen-reader action.
Under reduced motion gestures do not follow the finger; passing the threshold acts at once.

#### Scenario: A gesture without a finger
- **WHEN** a person who cannot swipe wants to reply to a message or go back
- **THEN** the long-press menu has Trả lời / Trích dẫn and the top bar has ←
