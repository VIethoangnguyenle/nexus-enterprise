# Design — Chat Rebuild Tiers 2-5

## Design Approach

**Zero new components.** All UI is composed from existing Stitch primitives and chat composites.

## Component Mapping

### 1. Reactions — EmojiPicker Popup

**Trigger**: HoverActionBar "React" button click
**Component**: `EmojiPicker.tsx` (EXISTS — emoji-mart)
**Positioning**: Absolute, anchored to HoverActionBar, above message
**Behavior**:
- Open on React button click
- Close on emoji select or click-outside
- After select: call `onToggleReaction(emoji)`

**Reused primitives**: None new — EmojiPicker + HoverActionBar already exist

### 2. Reactions — ReactionBar Toggle

**Component**: `ReactionBar.tsx` (EXISTS)
**Enhancement needed**:
- Each reaction pill must be clickable → toggle reaction
- My reactions highlighted with `bg-primary/10 border-primary` vs `bg-surface-container border-outline-variant`
- "+" button at end opens EmojiPicker

**Design tokens**:
- Active pill: `bg-primary/10 text-primary border border-primary/30`
- Inactive pill: `bg-surface-container text-on-surface-variant border border-outline-variant/30`
- Size: `text-xs px-2 py-0.5 rounded-full`

### 3. Thread Reply Count Badge

**Location**: Below message content, before ReactionBar
**Design**: `text-primary text-xs cursor-pointer hover:underline flex items-center gap-1`
**Content**: `💬 N replies` or `MessageSquare icon + "N replies"`
**Behavior**: Click opens ThreadPanel

### 4. ThreadPanel Reply Editor

**Location**: Bottom of ThreadPanel
**Component**: Simplified ChatEditor or plain `<input>` with send button
**Design**: `border-t border-outline-variant p-3` at bottom of PeekPanel
**Placeholder**: "Reply..."

### 5. Pin Indicator on Messages

**Location**: After timestamp in message header
**Design**: `Pin` icon (lucide), 12px, `text-on-surface-variant`
**Visibility**: Only when `message.is_pinned === true`

### 6. Pins Tab in ChannelInfoPanel

**Existing tab**: "Pins" tab already exists in ChannelInfoPanel
**Content**: List of pinned messages with:
- Message content preview (truncated)
- Sender name + avatar
- "Pinned by [name] on [date]"
**Empty state**: "No pinned messages" with Pin icon

### 7. Search Tab in ChannelInfoPanel

**Existing tab**: "Search" tab already exists in ChannelInfoPanel
**Layout**:
- Search input at top: `bg-surface-container rounded-lg px-3 py-2`
- Results list below: message cards with sender, content highlight, timestamp
**Empty state**: "No results found"

### 8. Member Remove Button

**Location**: Member row in ChannelInfoPanel, hover reveal
**Design**: `X` or `UserMinus` icon, `text-on-surface-variant hover:text-error`
**Confirmation**: Standard confirm dialog
**Hidden for**: Current user (cannot remove self)

### 9. @Mention Autocomplete

**Location**: Above cursor position in ChatEditor
**Design**: Floating dropdown with:
- `bg-surface-container-lowest border border-outline-variant rounded-lg shadow-lg`
- Each item: Avatar (sm) + display name
- Selected item: `bg-surface-container`
- Max height: 200px, scrollable

## New Components: 0
## Reused Components: EmojiPicker, ReactionBar, HoverActionBar, ThreadPanel, PeekPanel, ChatEditor, ChannelInfoPanel, MessageList, Avatar, Text, Spinner
