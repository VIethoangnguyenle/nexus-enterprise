# Reactions — Spec

## User Stories

### S1: Toggle Emoji Reaction
As a workspace member viewing a message,
I want to add an emoji reaction via hover action bar,
so that I can express sentiment without typing.

Acceptance Criteria:
- [ ] Hovering a message shows HoverActionBar with "React" button
- [ ] Clicking "React" opens EmojiPicker popup positioned near the message
- [ ] Selecting an emoji adds the reaction below the message instantly (optimistic)
- [ ] Clicking the same reaction on a message I already reacted to removes it (toggle)
- [ ] Other users see the reaction appear in real-time via WebSocket
- [ ] Reaction count updates correctly (increment on add, decrement on remove)
- [ ] Clicking outside EmojiPicker closes it
- [ ] Empty state: message with no reactions shows no ReactionBar

Proto mapping: `POST /messages/:id/reactions` + `DELETE /messages/:id/reactions/:emoji` (REST — EXISTS)
WS event: `reactionEvent` (EXISTS in ws.proto)
Frontend-only: YES — API + hooks + WS handler + ReactionBar + EmojiPicker all exist

### S2: Quick Reaction from Existing
As a workspace member viewing reactions on a message,
I want to click an existing reaction emoji to add/remove my vote,
so that I can react quickly without opening the full picker.

Acceptance Criteria:
- [ ] Each reaction pill in ReactionBar is clickable
- [ ] Clicking a reaction I haven't used adds me (count +1, blue highlight)
- [ ] Clicking a reaction I already used removes me (count -1, no highlight)
- [ ] If count reaches 0, reaction pill disappears
- [ ] My own reactions are visually distinguished (blue/primary highlight)

Proto mapping: Same as S1
Frontend-only: YES

## Flow

### Add Reaction Flow
1. User hovers message → HoverActionBar appears
2. User clicks "React" (SmilePlus icon)
3. EmojiPicker popup opens above/below the action bar
4. User clicks emoji (e.g. 👍)
5. System calls `POST /messages/:id/reactions` { emoji: "👍" }
6. Optimistic: ReactionBar updates immediately
7. WS `reactionEvent` broadcasts to other users
8. EmojiPicker closes

### Toggle Reaction Flow
1. User sees existing reaction "👍 2" on a message
2. User clicks the "👍 2" pill
3. System toggles: if user reacted → remove, else → add
4. Optimistic update

## States
- **Empty**: No reactions → no ReactionBar rendered
- **Loading**: N/A (optimistic)
- **Active**: Reactions visible below message content
- **Error**: If API fails, revert optimistic update
