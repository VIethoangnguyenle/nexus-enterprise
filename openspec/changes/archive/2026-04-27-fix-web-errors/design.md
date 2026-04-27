# Design — Fix Web Errors

## Root Causes

| # | Error | Root Cause | Fix Layer |
|---|-------|-----------|-----------|
| 1 | `relation "drive_items" does not exist` | init.sql only runs on fresh PG volumes; drive tables added after volume existed | Makefile + docs |
| 2 | `drive root not found` | CreateWorkspace → CreateDriveForChannel failed silently when drive tables didn't exist | Backend: workspace service |
| 3 | Channel drive 400 | Gateway `handleDriveChannelDrive` passes empty channel drive context | Backend: gateway handler |
| 4 | Documents upload uses deprecated flow | `documentApi.create()` bypasses presigned URL workflow | Frontend: documents page |
| 5 | Empty `wsId` causes bad API calls | `workspaces[0]?.id || ''` fires API with `//` in path | Frontend: guard |
| 6 | No workspace creation UI | Only way to create workspace is via curl | Frontend: onboarding |

## Fix Strategy

### Fix 1: Database Migration Safety Net
- `make db-migrate` already added — safe idempotent re-apply of init.sql
- Add `make deploy` to auto-run `db-migrate` after services start

### Fix 2: Self-healing Drive Root
- In `DriveServer.CreateFolder` and `DriveServer.ListFolder`: when workspace root not found, **auto-create it** instead of returning NotFound
- Lookup workspace's Documents OA from NGAC graph and use as the root NGAC node

### Fix 3: Channel Drive Error Handling
- Gateway `handleDriveChannelDrive`: if channel drive doesn't exist yet, auto-create via `CreateDriveForChannel` RPC before returning
- Return empty items `[]` for new channels instead of 400

### Fix 4: Documents Upload
- Replace `documentApi.create()` with presigned URL 3-step workflow matching Drive pattern
- Or redirect Documents page to Drive page (since Drive is the new canonical storage)

### Fix 5: Empty wsId Guard
- Add guard in `_workspace.tsx`: if no workspaces, show onboarding instead of rendering pages with empty wsId
- All hooks that take wsId should use `enabled: !!wsId` to prevent bad API calls

### Fix 6: Workspace Onboarding
- Simple "Create Workspace" card when `workspaces.length === 0`
- Input field + button, uses `useCreateWorkspace()` hook

## Decision: Documents vs Drive

Documents page currently duplicates Drive functionality. Two options:
1. **Keep both** — Documents for legacy file storage, Drive for new hierarchical storage
2. **Merge** — Redirect `/documents` to `/drive`, remove legacy Documents page

**Decision**: Keep both for now. Documents page uses Document Service (flat list). Drive uses Drive Service (hierarchical). Fix Documents upload to work correctly.
