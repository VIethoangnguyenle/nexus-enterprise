# Chat Member Management — Overview

## Module
Messaging → Channel Members

## Proto Mapping

| RPC | Status | REST Endpoint |
|-----|--------|---------------|
| `AddChannelMember` | ✅ gRPC + REST `POST /channels/:chId/members` | Exists |
| `RemoveChannelMember` | ⚠️ gRPC only, REST missing | Need `DELETE /channels/:chId/members/:nodeId` |
| `ListChannelMembers` | ✅ gRPC + REST `GET /channels/:chId/members` | Exists |

## Dependencies

| Dependency | Status |
|------------|--------|
| Proto: `messaging.proto` member types | ✅ Complete |
| Backend: domain `AddMember`, `RemoveMember`, `ListMembers` | ✅ Complete |
| Backend: REST `AddChannelMember`, `ListChannelMembers` | ✅ Complete |
| Backend: REST `RemoveChannelMember` | ❌ Missing — must add |
| Frontend: `messaging.ts` API client `addMember`, `removeMember` | ❌ Missing |
| Frontend: `useMessaging.ts` mutation hooks | ❌ Missing |
| Frontend: `ChannelInfoPanel.tsx` MembersTab UI | ⚠️ Read-only |
| Frontend: Workspace contacts for user picker | ✅ `useContacts()` available |

## Capabilities

1. **Add Member** — Search workspace members, add to channel
2. **Remove Member** — Remove member from channel (admin/owner action)
