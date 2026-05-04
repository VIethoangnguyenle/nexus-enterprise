# Tasks: Platform-Wide WebSocket Cache Injection

## WebSocket Store Handlers
- [ ] T1. Add `reactionEvent` handler with cache injection
- [ ] T2. Add `pinEvent` handler with cache injection
- [ ] T3. Add `pollVoteEvent` handler (pollVote) with cache injection
- [ ] T4. Add `taskUpdateEvent` handler (taskUpdate) with cache injection
- [ ] T5. Refine `driveObject` handler — scope invalidation to specific folder
- [ ] T6. Clean up `default` handler — remove broad invalidations

## Mutation Hooks (useMessaging.ts)
- [ ] T7. Convert `useToggleReaction` to optimistic update
- [ ] T8. Convert `useTogglePin` to optimistic update
- [ ] T9. Convert `useVotePoll` to optimistic update
- [ ] T10. Convert `useCreateTask` to optimistic update
- [ ] T11. Convert `useUpdateTask` to optimistic update
- [ ] T12. Remove `refetchInterval: 30000` from `useUnreadCounts`

## Mutation Hooks (useDrive.ts)
- [ ] T13. Convert `useTrashItem` + `useDeleteItem` to optimistic removal

## Verification
- [ ] T14. Build check (`vite build`)
