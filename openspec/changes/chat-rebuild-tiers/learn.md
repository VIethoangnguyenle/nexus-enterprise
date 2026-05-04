# LEARN: Chat Rebuild Tiers 2-5

## Key Learnings

### 1. Codebase Maturity Assessment First
Before writing any code, scan existing APIs, hooks, stores, and components.
In this case, 90%+ of the work was already done — only 2 actual gaps existed (ThreadPanel reply editor, @mention autocomplete).

**Pattern**: `grep + view` before `write`.

### 2. Thread 404 is Expected Behavior
Backend messaging service returns 404 for `/messages/:id/thread` when a message has zero replies.
Frontend MUST handle this as "no replies yet" rather than showing an error state.

**Fix**: Check `error.message.includes('Not Found')` before rendering ErrorState.

### 3. Overflow-hidden Clips Absolute Positioned Children
When placing dropdowns/popups inside a container with `overflow-hidden` (common in flex layouts with scroll), the popup gets clipped.

**Fix**: Position the popup on a parent container that doesn't have `overflow-hidden`, and use `z-50` for proper stacking.

### 4. Tiptap @mention Detection Pattern
```typescript
onUpdate: ({ editor: ed }) => {
  const { from } = ed.state.selection
  const textBefore = ed.state.doc.textBetween(Math.max(0, from - 20), from, '\n')
  const mentionMatch = textBefore.match(/@(\w*)$/)
  // mentionMatch[1] = query string after @
}
```

### 5. Thread Reply API Pattern
Thread replies use the same `sendMessage` endpoint with `parent_message_id`:
```typescript
messagingApi.sendMessage(channelId, {
  content: text,
  content_format: 'text',
  parent_message_id: messageId,
})
```
After sending, invalidate both `['thread', messageId]` and `['messages', channelId]`.

### 6. Optimistic UI Rollback is Correct
When backend returns errors (502, 404, etc.), the optimistic update correctly rolls back via `onError` handler.
This is NOT a frontend bug — it's the expected graceful degradation pattern.

## Architecture Constraints Maintained
- No new routes
- No new stores
- No new backend endpoints
- All M3 design tokens
- All existing primitives reused
