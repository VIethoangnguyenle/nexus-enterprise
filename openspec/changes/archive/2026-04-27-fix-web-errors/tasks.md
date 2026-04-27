# Tasks — Fix Web Errors

## Phase 1: Backend Stability

- [x] 1.1 Drive Service: auto-create workspace drive root when not found (self-healing)
- [x] 1.2 Drive Service: auto-create channel drive root on first access (lazy creation)
- [x] 1.3 Gateway: return `{"items":[]}` instead of 400 for missing channel drive
- [x] 1.4 Gateway: validate `workspaceId` param is non-empty in drive handlers
- [x] 1.5 Makefile: add `db-migrate` step to `deploy` target

## Phase 2: Frontend Guards

- [x] 2.1 `_workspace.tsx`: if `workspaces.length === 0`, show "Create Workspace" onboarding
- [x] 2.2 All drive/document hooks: `enabled: !!wsId` (already present)
- [x] 2.3 Messaging hooks: `enabled: !!channelId` (already present)

## Phase 3: Frontend Error Fixes

- [x] 3.1 Documents page: verified presigned URL flow is correct
- [x] 3.2 Drive page: replaced alert() with console.error
- [x] 3.3 Channel view: ChannelDrivePanel handles errors gracefully

## Phase 4: Verify

- [x] 4.1 `go build` — drive + gateway compile ✓
- [x] 4.2 Deploy — drive + gateway redeployed successfully ✓
- [x] 4.3 Browser test: login → Drive (no errors) → Channel (no errors) → Documents (no errors) ✓
- [x] 4.4 Verified no blocking errors on fresh session ✓
