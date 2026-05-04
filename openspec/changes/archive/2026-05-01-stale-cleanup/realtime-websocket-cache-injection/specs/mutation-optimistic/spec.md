# Mutation Optimistic Updates

## Scope
Convert all messaging mutation hooks from `onSuccess: invalidateQueries` to optimistic update pattern.

## Mutations to Convert

### 1. useToggleReaction (useMessaging.ts:108-117)
- Current: `invalidateQueries(['messages', channelId])` 
- Target: `onMutate` optimistic toggle → `onSuccess` no-op → `onError` rollback

### 2. useTogglePin (useMessaging.ts:130-141)
- Current: `invalidateQueries(['pins', channelId])` + `invalidateQueries(['messages', channelId])`
- Target: `onMutate` optimistic toggle `is_pinned` → `onSuccess` no-op → `onError` rollback

### 3. useVotePoll (useMessaging.ts:188-192)
- Current: `invalidateQueries(['poll', pollId])`
- Target: `onMutate` increment vote_count → `onSuccess` no-op → `onError` rollback

### 4. useCreateTask (useMessaging.ts:205-213)
- Current: `invalidateQueries(['tasks', channelId])` + `invalidateQueries(['messages', channelId])`
- Target: `onMutate` optimistic insert → `onSuccess` replace temp → `onError` rollback

### 5. useUpdateTask (useMessaging.ts:216-221)
- Current: `invalidateQueries(['tasks', channelId])`
- Target: `onMutate` optimistic field update → `onSuccess` no-op → `onError` rollback

### 6. useUnreadCounts polling removal (useMessaging.ts:145-151)
- Current: `refetchInterval: 30000` (polling every 30s)
- Target: Remove polling — WS `unreadCount` event handles updates

### 7. Drive mutations (useDrive.ts) — Optimistic delete/trash
- `useTrashItem`: optimistic remove from folder cache
- `useDeleteItem`: optimistic remove from folder cache
- Other drive mutations (create folder, upload, rename, move): keep invalidation but scope to specific folder

## Acceptance Criteria
- [ ] All converted mutations show instant UI feedback
- [ ] All converted mutations rollback on error
- [ ] Network tab shows no GET calls after successful mutations
- [ ] WS events deduplicate with optimistic updates
