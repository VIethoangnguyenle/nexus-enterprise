# Architecture — Chat Rebuild Tiers 2-5

## Architecture Decision

**All changes are FRONTEND-ONLY.** No backend, proto, DB, or service changes needed.

### Rationale
Every API endpoint, WebSocket event handler, and React hook already exists:
- REST APIs: reactions, pins, threads, search, members — all in `api/messaging.ts`
- Hooks: `useReaction`, `useTogglePin`, `useThread`, `usePins` — all in `hooks/useMessaging.ts`
- WS handlers: `reactionEvent`, `threadReply`, `pinEvent` — all in `stores/websocket.store.ts`
- Components: `EmojiPicker`, `ReactionBar`, `HoverActionBar`, `ThreadPanel` — all in `components/chat/`

### Data Flow (unchanged)

```
User Action → React Component → TanStack Mutation (optimistic) → REST API → Backend
                                                                     ↓
                                                              WebSocket broadcast
                                                                     ↓
                                         WS Store handler → TanStack QueryClient cache injection
```

### Files Modified (by capability)

| Capability | Files | Type |
|-----------|-------|------|
| Reactions | `MessageList.tsx`, `channels.$channelId.tsx` | Wire EmojiPicker popup + reaction toggle |
| Threads | `MessageList.tsx`, `ThreadPanel.tsx` | Add reply count badge + reply editor |
| Pins | `ChannelInfoPanel.tsx`, `MessageList.tsx` | Pin list tab + pin indicator |
| Search | `ChannelInfoPanel.tsx` | Search tab with results |
| Members | `ChannelInfoPanel.tsx` | Remove member button + confirmation |
| Mentions | `ChatEditor.tsx` | @mention autocomplete dropdown |

### NGAC Permissions
- Member removal: Backend already checks NGAC permissions on `DELETE /channels/:id/members/:nodeId`
- All other actions: Backend validates membership on all API calls
- No new permission checks needed in frontend

### Integration Points
- No new gRPC calls
- No new REST endpoints
- No new WebSocket event types
- No new proto changes

### Constraints
- EmojiPicker popup: must position correctly relative to HoverActionBar
- ThreadPanel: already uses PeekPanel, must not conflict with ChannelInfoPanel
- @mention dropdown: must position relative to cursor in ChatEditor (Tiptap)
