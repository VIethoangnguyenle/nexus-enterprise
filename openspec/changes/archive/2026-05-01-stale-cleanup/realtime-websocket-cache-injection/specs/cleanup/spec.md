# Default Handler Cleanup

## Current Problem
The `default` case in `handleServerMessage` catches ALL unhandled WS event types and fires 5 broad invalidations:
```typescript
queryClient.invalidateQueries({ queryKey: ['messages'] })
queryClient.invalidateQueries({ queryKey: ['reactions'] })
queryClient.invalidateQueries({ queryKey: ['pins'] })
queryClient.invalidateQueries({ queryKey: ['polls'] })
queryClient.invalidateQueries({ queryKey: ['tasks'] })
```

This means `reactionEvent`, `pinEvent`, `pollVoteEvent`, `taskUpdateEvent` currently trigger BOTH their specific handler AND the default case.

## Changes Required

### 1. Add explicit handlers for all WS event types
After adding handlers for reaction, pin, poll, task events, the default case should only fire for truly unknown event types.

### 2. Reduce default handler scope
Change the default handler to:
```typescript
default:
  if (envelope.payload.oneofKind && WS_DEBUG()) {
    console.warn('[WS] unhandled event type:', envelope.payload.oneofKind)
  }
  break
```

No broad invalidation — if an event type isn't handled, log it (debug only).

## Acceptance Criteria
- [ ] All 15 ServerEnvelope event types have explicit `case` handlers
- [ ] Default handler does NOT fire `invalidateQueries`
- [ ] Default handler logs unhandled types in debug mode only
- [ ] No regression: all existing WS events still work
