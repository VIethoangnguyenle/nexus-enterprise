## Architecture

The WebSocket real-time flow requires three things to work end-to-end:

```
1. WS Connection    ✅  Browser → ws://host/api/ws → Hub (101 Upgrade)
2. Channel Subscribe ❌  Browser sends subscribe(channelId) → Hub.channels[id][client] = true
3. Broadcast         ✅  REST handler → hub.BroadcastToChannel → broadcastLocal → client.send
```

Step 2 is missing. Without it, `broadcastLocal()` finds zero clients and drops the message.

## Design Decision

### Subscribe/Unsubscribe lifecycle in channel route

**Decision**: Add `useEffect` in `channels.$channelId.tsx` that subscribes on mount and unsubscribes on cleanup.

```tsx
const sendSubscribe = useWebSocketStore(s => s.sendSubscribe)
const sendUnsubscribe = useWebSocketStore(s => s.sendUnsubscribe)

useEffect(() => {
  sendSubscribe(channelId)
  return () => sendUnsubscribe(channelId)
}, [channelId, sendSubscribe, sendUnsubscribe])
```

**Rationale**: 
- Subscribe when the user enters a channel, unsubscribe when they leave
- The WS store already tracks `subscribedChannels` (Set) and re-subscribes on reconnect
- `sendSubscribe` is safe if WS isn't connected — it adds to the Set and sends when authenticated
- This matches the existing `sendTyping` pattern already in the component

## File Change

### `frontend/src/routes/_workspace/channels.$channelId.tsx`

Add after existing useEffect hooks (around line 67):

```diff
+  const sendSubscribe = useWebSocketStore(s => s.sendSubscribe)
+  const sendUnsubscribe = useWebSocketStore(s => s.sendUnsubscribe)
+
+  // Subscribe to WS channel for real-time message delivery.
+  useEffect(() => {
+    sendSubscribe(channelId)
+    return () => sendUnsubscribe(channelId)
+  }, [channelId, sendSubscribe, sendUnsubscribe])
```

## Verification

1. Open two browser tabs at the same channel
2. Send a message in Tab A → should appear in Tab B without reload
3. Navigate to a different channel → should unsubscribe from old channel
4. Navigate back → should re-subscribe
