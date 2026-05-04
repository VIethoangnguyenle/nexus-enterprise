# Design: Fix Channel Creation Access Denied

## Fix 1: Fallback NGAC node lookup (P0)

### Problem
`assignChannelToWorkspace` calls `findChildByName(ctx, ws.PCNodeID, ngac.ChannelsOAName(workspaceID), ngac.TypeOA)` which generates `"6c997738-..._Channels"`. Old workspaces have nodes named `"hoang_Channels"`.

### Solution
Add a fallback in `assignChannelToWorkspace`: if ID-based lookup fails, try name-based lookup using `ws.Name`.

#### File: `backend/services/messaging/internal/domain/service.go`

```diff
 func (s *Service) assignChannelToWorkspace(ctx context.Context, workspaceID, contentOAID, membersUAID string) error {
 	if workspaceID == "" {
 		return s.assignToGlobalPC(ctx, contentOAID, membersUAID)
 	}

 	ws, err := s.store.GetWorkspaceByID(ctx, workspaceID)
 	if err != nil || ws == nil {
 		return fmt.Errorf("workspace lookup failed: %w", err)
 	}

-	channelsOAID := s.findChildByName(ctx, ws.PCNodeID, ngac.ChannelsOAName(workspaceID), ngac.TypeOA)
+	// Try ID-based naming first (new convention), fallback to name-based (legacy).
+	channelsOAID := s.findChildByName(ctx, ws.PCNodeID, ngac.ChannelsOAName(workspaceID), ngac.TypeOA)
+	if channelsOAID == "" {
+		channelsOAID = s.findChildByName(ctx, ws.PCNodeID, ngac.ChannelsOAName(ws.Name), ngac.TypeOA)
+	}
 	if channelsOAID != "" {
 		s.policyWrite.CreateAssignment(ctx, &policypb.CreateAssignmentRequest{
 			ChildId: contentOAID, ParentId: channelsOAID,
 		})
 	}
```

**Why this approach**: Zero migration risk. Works for both old and new workspaces. The fallback only fires when the primary lookup misses.

---

## Fix 2: Channel type constraint (P1)

### Problem
Frontend sends `channel_type: "group"` but DB CHECK constraint only allows `workspace | private | dm`.

### Solution
Normalize `channel_type` in the domain layer before DB insert.

#### File: `backend/services/messaging/internal/domain/service.go`

In `CreateChannel`, before `InsertChannel`:
```go
// Normalize channel type: "group" maps to "workspace" for DB constraint.
if in.ChannelType == "group" {
    in.ChannelType = "workspace"
}
```

**Why domain layer**: Input validation belongs in domain, not store. The mapping is a business decision — "group" and "workspace" channels have the same NGAC behavior.

---

## Fix 3: Access denied error mapping (P2)

### Problem
`checkAccess` returns `fmt.Errorf("access denied: ...")` which is a generic error. The REST handler maps it to HTTP 500 instead of 403.

### Solution
Use `domain.ErrAccessDenied` sentinel error and ensure `mapError` in REST handler catches it.

#### File: `backend/services/messaging/internal/domain/service.go`

```diff
 func (s *Service) checkAccess(ctx context.Context, userNodeID, objectNodeID, operation string) error {
 	resp, _ := s.policyRead.CheckAccess(ctx, &policypb.CheckAccessRequest{
 		UserNodeId: userNodeID, ObjectNodeId: objectNodeID, Operation: operation,
 	})
 	if resp != nil && resp.Decision == ngac.DecisionDeny {
-		return fmt.Errorf("access denied: %s on %s", operation, objectNodeID)
+		return fmt.Errorf("%w: %s on %s", ErrAccessDenied, operation, objectNodeID)
 	}
 	return nil
 }
```

#### File: `backend/services/messaging/internal/domain/errors.go`

Ensure `ErrAccessDenied` sentinel exists (it should already from the architecture rules).

#### File: `backend/services/messaging/internal/rest/handler.go`

Ensure `mapError` handles `domain.ErrAccessDenied` → HTTP 403.

---

## Verification

1. **API test**: Create channel → GET messages → expect 200 (not 500)
2. **API test**: Create channel with `channel_type: "group"` → expect 200
3. **API test**: Access denied → expect HTTP 403 (not 500)
4. **Regression**: Existing "Test" channel still works
5. **Go build**: `go build ./cmd/` passes for messaging service
