# Workspace Owners Array Report

- Date: 2026-10-11
- Plan: `plans/261010-2258-personal-workspace/exec-07-owner-names.md`
- Executor: agy (Antigravity)
- Mode: edit source, no commits, no git index changes, no service restarts.

## Overview

Implemented `owner_names: string[]` in `GET /api/workspaces/:id/details` and updated frontend `WorkspaceTab.tsx` to consume `owner_names` with backward compatibility fallback to splitting `owner_name`. This prevents display names containing commas from being erroneously split into multiple PersonChips.

## Changes Implemented

### 1. Backend Domain (`backend/services/workspace/internal/domain/workspace_details.go`)
- Added `OwnerNames []string` to domain struct `WorkspaceDetails`.
- Populated `d.OwnerNames = names` in `withOwnership` alongside `d.OwnerName = strings.Join(names, ", ")`.
- Added unit test `TestWorkspaceDetails_OwnerNamesIncludesNamesWithComma` in `workspace_details_test.go` verifying multiple owners and names with commas. Asserted 403 access denial for non-members in `TestWorkspaceDetails_NonMembersAreDenied`.

### 2. Backend REST (`backend/services/workspace/internal/rest/workspace_details_handler.go`)
- Added `OwnerNames []string` (`json:"owner_names"`) to `workspaceDetailsJSON`.
- Updated `detailsJSON` to output `owner_names` (defaulting to empty slice if nil).
- Updated `handler_test.go` with tests verifying `owner_names` JSON serialization, preservation of comma in owner display name, and 403 forbidden for non-members.

### 3. Documentation (`docs/specs/tenant-identity/spec.md`)
- Documented `owner_names` array in `GET /api/workspaces/{id}/details` response specification.

### 4. Frontend API (`frontend/src/api/workspaces.ts`)
- Added `owner_names?: string[]` to `WorkspaceDetails` interface.

### 5. Frontend UI (`frontend/src/components/settings/WorkspaceTab.tsx`)
- Updated `WorkspaceTypeCard` to read `details.owner_names` when present, falling back to splitting `details.owner_name` on `", "` only when absent.
- Added test in `SettingsScreen.test.tsx` verifying that a name containing a comma renders as a single PersonChip when `owner_names` is provided.

## Verification & Gates
- Backend Go tests & vet:
  `cd backend/services/workspace && go test ./... && go vet ./...` -> PASS
- Layering check:
  `make check-layering` -> PASS
- Frontend tests:
  `cd frontend && npx vitest run src/components/settings/SettingsScreen.test.tsx` -> PASS (45/45)
  `cd frontend && npx vitest run src/components/spaces/SpaceView.test.tsx` -> PASS (20/20)
  Full vitest run: 175/177 suites passed (only known pre-existing AssetsScreen failure)
- Linter:
  `cd frontend && npx eslint src/api/workspaces.ts src/components/settings/WorkspaceTab.tsx src/components/settings/SettingsScreen.test.tsx` -> PASS (0 errors, 0 warnings)
- TypeScript:
  `cd frontend && npm run typecheck:diff` -> PASS (no new errors vs baseline)
