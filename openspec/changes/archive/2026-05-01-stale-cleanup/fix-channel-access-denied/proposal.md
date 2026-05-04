# Fix Channel Creation "Access Denied"

## Problem

When a user creates a new channel in any **pre-migration workspace**, they immediately receive `access denied: read on <OA_ID>`. This is a **P0** — the channel creation feature is completely broken for all existing workspaces.

## Root Cause

NGAC naming inconsistency between workspace creation and channel creation.

**Old workspaces** (created before ID-based naming convention) have NGAC nodes named by workspace **display name**:
```
PC_hoang, hoang_Channels, hoang_Members, hoang_Owners
```

**Current code** (`ngac.ChannelsOAName(wsID)`) searches by workspace **UUID**:
```
6c997738-698a-45ba-a403-5ee0cc1d7781_Channels → NOT FOUND
```

When `findChildByName` returns empty, the channel's Content OA is never assigned into the workspace NGAC tree → orphan node → policy engine denies all access.

**Secondary bug**: Frontend sends `channel_type: "group"` but DB CHECK constraint only allows `workspace | private | dm`.

## Scope

### In scope
- Fix NGAC node lookup to work with both old (name-based) and new (ID-based) naming
- Fix `channel_type` validation to handle the "group" type
- Fix `checkAccess` error mapping (currently returns 500, should return 403)

### Out of scope
- Full NGAC node migration to ID-based naming (future change)
- `/api/auth/me` endpoint (separate fix)
- Nested button HTML warning (cosmetic)

## Impact
- **Services affected**: Messaging (primary), Policy (read path only)
- **Risk**: Low — additive fallback, no destructive changes
- **Users affected**: All users in workspaces created before ID-based naming migration
