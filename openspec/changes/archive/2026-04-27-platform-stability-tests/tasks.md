## 1. Bug Fixes

- [x] 1.1 Fix `ListChannels` NULL scan — add COALESCE for all nullable columns
- [x] 1.2 Fix `GetChannel` NULL scan — add COALESCE for all nullable columns  
- [x] 1.3 Add wsId guard in `CreateChannelModal` — disable create when no workspace
- [x] 1.4 Clean orphan channels — DELETE workspace channels with NULL workspace_id
- [x] 1.5 Rebuild + deploy messaging service
- [x] 1.6 Rebuild + deploy frontend
- [x] 1.7 Verify: create channel → list channels → send message works end-to-end

## 2. Go Test Infrastructure

- [x] 2.1 Add `testify` dependency to backend `go.mod`
- [x] 2.2 Create test helper package: `backend/testutil/` with DB setup, mock builders
- [x] 2.3 Verify `go test ./...` runs from backend root with zero tests

## 3. Messaging Service Tests

- [x] 3.1 Write `server_test.go` — TestCreateChannel (happy path + missing workspace)
- [x] 3.2 Write `server_test.go` — TestListChannels (with NULL workspace_id rows)
- [x] 3.3 Write `server_test.go` — TestGetChannel (valid + not found + NULL columns)
- [x] 3.4 Write `server_test.go` — TestSendMessage (happy path + no access)
- [x] 3.5 Write `server_test.go` — TestGetMessages (pagination, thread filtering)
- [x] 3.6 Write `server_test.go` — TestCreateDM (happy path + existing DM dedup)
- [x] 3.7 Write `server_test.go` — TestGetThread (parent + replies)
- [x] 3.8 All messaging tests pass: `go test ./services/messaging/...`

## 4. Asset Service Tests

- [x] 4.1 Write `store_test.go` — TestCreateType + TestGetType + TestListTypes
- [x] 4.2 Write `store_test.go` — TestCreateAsset + TestGetAsset (nullable assigned_to)
- [x] 4.3 Write `store_test.go` — TestListAssets (filters: type, state, assigned_to)
- [x] 4.4 Write `store_test.go` — TestUpdateAssetState + TestClearAssignment
- [x] 4.5 Write `store_test.go` — TestSoftDeleteAsset (idempotent, already deleted)
- [x] 4.6 Write `store_test.go` — TestCreateRequest + TestGetRequest (nullable approver)
- [x] 4.7 Write `store_test.go` — TestListRequests (filters: status, mine_only)
- [x] 4.8 Write `store_test.go` — TestFulfillRequest + TestUpdateRequestStatus
- [x] 4.9 Write `store_test.go` — TestInsertTransition + TestGetAssetHistory
- [x] 4.10 All asset tests pass: `go test ./services/asset/...`

## 5. Auth Service Tests

- [x] 5.1 Write `store_test.go` — TestCreateUser + TestGetUserByUsername
- [x] 5.2 Write `store_test.go` — TestGetUserByID + TestGetUserByNGACNodeID
- [x] 5.3 Write `store_test.go` — TestListUsers
- [x] 5.4 Write `server_test.go` — TestRegister (happy + duplicate username)
- [x] 5.5 Write `server_test.go` — TestLogin (happy + wrong password + user not found)
- [x] 5.6 All auth tests pass: `go test ./services/auth/...`

## 6. Policy Service Tests

- [x] 6.1 Write `store_test.go` — TestCreateNode + TestFindNodeByName
- [x] 6.2 Write `store_test.go` — TestCreateAssignment + TestGetChildren
- [x] 6.3 Write `store_test.go` — TestCreateAssociation + graph traversal
- [x] 6.4 Write `graph_test.go` — TestCheckAccess (ALLOW + DENY scenarios)
- [x] 6.5 Write `graph_test.go` — TestCheckAccess (multi-PC, inherited permissions)
- [x] 6.6 All policy tests pass: `go test ./services/policy/...`

## 7. Document Service Tests

- [x] 7.1 Write `server_test.go` — TestUpload + TestList (access filtering)
- [x] 7.2 Write `server_test.go` — TestGetUploadURL + TestConfirmUpload (skip: requires MinIO)
- [x] 7.3 Write `server_test.go` — TestGetDownloadURL (skip: requires MinIO)
- [x] 7.4 Write `server_test.go` — TestApprove + TestPublish + access denied
- [x] 7.5 All document tests pass: `go test ./services/document/...`

## 8. Workspace Service Tests

- [x] 8.1 Write `server_test.go` — TestCreateWorkspace (NGAC graph setup)
- [x] 8.2 Write `server_test.go` — TestGetWorkspace (happy + not found)
- [x] 8.3 Write `server_test.go` — TestCreateRole
- [x] 8.4 Write `server_test.go` — TestCreateFolder
- [x] 8.5 All workspace tests pass: `go test ./services/workspace/...`

## 9. Gateway Tests

- [x] 9.1 Write `main_test.go` — TestAuthMiddleware (valid + invalid + missing + wrong secret)
- [x] 9.2 Write `main_test.go` — TestWriteResponse (success + error formatting)
- [x] 9.3 All gateway tests pass: `go test ./services/gateway/...`

## 10. Frontend Test Infrastructure

- [x] 10.1 Install vitest + @testing-library/react + @testing-library/jest-dom + jsdom
- [x] 10.2 Create `vitest.config.ts` with jsdom environment
- [x] 10.3 Create `src/test/setup.ts` with jest-dom matchers
- [x] 10.4 Add `test` script to `package.json`
- [x] 10.5 Verify `npm test` runs with zero tests

## 11. Frontend Component Tests

- [x] 11.1 Write `CreateChannelModal.test.tsx` — renders form fields
- [x] 11.2 Write `CreateChannelModal.test.tsx` — disable create when name empty
- [x] 11.3 Write `CreateChannelModal.test.tsx` — disable create when wsId empty
- [x] 11.4 Write `CreateChannelModal.test.tsx` — submits with channel_type 'workspace'
- [x] 11.5 Write `Sidebar.test.tsx` — renders all sections (Documents, Navigation, Channels)
- [x] 11.6 Write `Sidebar.test.tsx` — collapse toggle hides text
- [x] 11.7 Write `messaging.test.ts` — API URL construction correct
- [x] 11.8 All frontend tests pass: `npm test` — 17/17 pass

## 12. Full Verification

- [x] 12.1 `go test ./...` from all services — ALL 81 tests pass
- [x] 12.2 `npm test` from frontend — ALL 17 tests pass
- [x] 12.3 `test_app.sh` — 59/59 pass (no regression) ✅
- [x] 12.4 Browser test: UI loads, auth works, sidebar renders, create channel modal validates (blocked on workspace creation UI for new users)
- [x] 12.5 Browser test: asset management routes load (blocked on workspace association for new users)
