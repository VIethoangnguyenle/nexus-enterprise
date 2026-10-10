package grpc_test

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"ngac-platform/ngac"
	"ngac-platform/pkg/grpcauth"
	pb "ngac-platform/proto/drive"
	policypb "ngac-platform/proto/policy"
	grpcserver "ngac-platform/services/drive/internal/grpc"
)

// rulePolicyRead answers CheckAccess from an explicit grant set, so a test can
// pin both the operation and the object a guard checks. Anything not granted
// is denied. Node names resolve to deterministic IDs ("oa:" + name).
type rulePolicyRead struct {
	mockPolicyRead
	mu      sync.Mutex
	grants  map[[3]string]bool // {user, object, op}
	failErr error              // when set, every access check errors
	checks  [][3]string
}

func newRulePolicy() *rulePolicyRead {
	return &rulePolicyRead{grants: map[[3]string]bool{}}
}

func (r *rulePolicyRead) grant(user, object, op string) { r.grants[[3]string{user, object, op}] = true }

// anyObject grants an operation on every object; used only where a test is not
// about that operation.
const anyObject = "*"

func oaID(name string) string { return "oa:" + name }

func (r *rulePolicyRead) CheckAccess(_ context.Context, req *policypb.CheckAccessRequest, _ ...grpc.CallOption) (*policypb.AccessDecision, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	key := [3]string{req.UserNodeId, req.ObjectNodeId, req.Operation}
	r.checks = append(r.checks, key)
	if r.failErr != nil {
		return nil, r.failErr
	}
	if r.grants[key] || r.grants[[3]string{req.UserNodeId, anyObject, req.Operation}] {
		return &policypb.AccessDecision{Decision: ngac.DecisionAllow}, nil
	}
	return &policypb.AccessDecision{Decision: ngac.DecisionDeny}, nil
}

func (r *rulePolicyRead) FindNodeByName(_ context.Context, req *policypb.FindNodeByNameRequest, _ ...grpc.CallOption) (*policypb.NGACNode, error) {
	return &policypb.NGACNode{Id: oaID(req.Name), Name: req.Name, NodeType: req.NodeType}, nil
}

// recordingPolicyWrite records node deletions so a denied revoke can be shown
// to have left the share's OA — the thing that actually grants access — alone.
type recordingPolicyWrite struct {
	mockPolicyWrite
	mu      sync.Mutex
	deleted []string
}

func (w *recordingPolicyWrite) DeleteNode(_ context.Context, req *policypb.DeleteNodeRequest, _ ...grpc.CallOption) (*policypb.Empty, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.deleted = append(w.deleted, req.NodeId)
	return &policypb.Empty{}, nil
}

func newServerWith(t *testing.T, pr policypb.PolicyReadServiceClient, pw policypb.PolicyWriteServiceClient) (*grpcserver.DriveServer, *pgxpool.Pool) {
	t.Helper()
	pool, err := pgxpool.New(context.Background(), testDBURL())
	if err != nil {
		t.Fatalf("connect to test DB: %v", err)
	}
	if err := pool.Ping(context.Background()); err != nil {
		t.Skipf("test DB not available: %v", err)
	}
	t.Cleanup(func() { pool.Close() })
	return grpcserver.NewDriveServer(pool, pr, pw, &mockDocStorage{}), pool
}

// sharedFolder creates a folder and a share on it using an allow-all server,
// returning the folder and share. The guarded call under test then runs on a
// second server whose policy is the one being exercised.
func sharedFolder(t *testing.T, name string) (*pb.DriveItem, *pb.ShareInfo) {
	t.Helper()
	srv, pool := setupServer(t)
	wsID := getTestWorkspaceID(t, pool)
	folder, err := srv.CreateFolder(asCaller("", "ngac-owner"), &pb.CreateFolderRequest{
		WorkspaceId: wsID, Name: name,
	})
	require.NoError(t, err)
	share, err := srv.CreateShare(asCaller("", "ngac-owner"), &pb.CreateShareRequest{
		ItemId: folder.Id, ShareType: "user", TargetNgacNodeId: "ngac-user-2",
		Operations: []string{ngac.OpRead},
	})
	require.NoError(t, err)
	t.Cleanup(func() { cleanDriveItems(t, pool, folder.Id) })
	return folder, share
}

func shareStillListed(t *testing.T, itemID, shareID string) bool {
	t.Helper()
	srv, _ := setupServer(t)
	list, err := srv.ListShares(asCaller("", "ngac-owner"), &pb.ListSharesRequest{ItemId: itemID})
	require.NoError(t, err)
	for _, s := range list.Shares {
		if s.Id == shareID {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// RevokeShare — allowed for the user who created the share, or for anyone
// holding share on the OA the item row points at.
//
// sharedFolder's share is created by "ngac-owner"; "ngac-admin" below is a
// different user, so the share-right tests do not pass via the creator path.
// ---------------------------------------------------------------------------

func TestRevokeShare_DeniedWithoutShareRight(t *testing.T) {
	folder, share := sharedFolder(t, "RevokeDeny")

	pr := newRulePolicy()
	// Write and read on the item are not enough: revoking takes share.
	pr.grant("ngac-intruder", folder.NgacNodeId, ngac.OpWrite)
	pr.grant("ngac-intruder", folder.NgacNodeId, ngac.OpRead)
	pw := &recordingPolicyWrite{}
	srv, _ := newServerWith(t, pr, pw)

	_, err := srv.RevokeShare(asCaller("", "ngac-intruder"), &pb.RevokeShareRequest{
		ShareId: share.Id,
	})

	require.Error(t, err)
	assert.Equal(t, codes.PermissionDenied, status.Code(err))
	assert.Empty(t, pw.deleted, "a denied revoke must not delete the share OA")
	assert.True(t, shareStillListed(t, folder.Id, share.Id), "a denied revoke must not delete the share row")
}

func TestRevokeShare_DeniedWithoutCaller(t *testing.T) {
	folder, share := sharedFolder(t, "RevokeNoCaller")

	pr := newRulePolicy()
	pw := &recordingPolicyWrite{}
	srv, _ := newServerWith(t, pr, pw)

	_, err := srv.RevokeShare(context.Background(), &pb.RevokeShareRequest{ShareId: share.Id})

	assert.Equal(t, codes.PermissionDenied, status.Code(err))
	assert.Empty(t, pw.deleted)
	assert.True(t, shareStillListed(t, folder.Id, share.Id))
}

func TestRevokeShare_DeniedWhenPolicyErrors(t *testing.T) {
	folder, share := sharedFolder(t, "RevokePolicyErr")

	pr := newRulePolicy()
	pr.grant("ngac-admin", folder.NgacNodeId, ngac.OpShare)
	pr.failErr = errors.New("policy unavailable")
	pw := &recordingPolicyWrite{}
	srv, _ := newServerWith(t, pr, pw)

	_, err := srv.RevokeShare(asCaller("", "ngac-admin"), &pb.RevokeShareRequest{
		ShareId: share.Id,
	})

	assert.Equal(t, codes.PermissionDenied, status.Code(err))
	assert.Empty(t, pw.deleted)
	assert.True(t, shareStillListed(t, folder.Id, share.Id))
}

func TestRevokeShare_AllowedWithShareRight(t *testing.T) {
	folder, share := sharedFolder(t, "RevokeAllow")

	pr := newRulePolicy()
	pr.grant("ngac-admin", folder.NgacNodeId, ngac.OpShare)
	pw := &recordingPolicyWrite{}
	srv, _ := newServerWith(t, pr, pw)

	_, err := srv.RevokeShare(asCaller("", "ngac-admin"), &pb.RevokeShareRequest{
		ShareId: share.Id,
	})

	require.NoError(t, err)
	assert.Contains(t, pr.checks, [3]string{"ngac-admin", folder.NgacNodeId, ngac.OpShare})
	assert.Len(t, pw.deleted, 1, "the share OA is what grants access and must be deleted")
	assert.False(t, shareStillListed(t, folder.Id, share.Id))
}

// A member creates shares with write (CreateShare's check) and holds no share
// op, yet must be able to withdraw a share they made.
func TestRevokeShare_CreatorCanRevokeOwnShareWithoutShareRight(t *testing.T) {
	folder, share := sharedFolder(t, "RevokeOwn")

	pr := newRulePolicy()
	pr.grant("ngac-owner", folder.NgacNodeId, ngac.OpWrite) // member-level rights only
	pw := &recordingPolicyWrite{}
	srv, _ := newServerWith(t, pr, pw)

	_, err := srv.RevokeShare(asCaller("", "ngac-owner"), &pb.RevokeShareRequest{
		ShareId: share.Id,
	})

	require.NoError(t, err)
	assert.Len(t, pw.deleted, 1)
	assert.False(t, shareStillListed(t, folder.Id, share.Id))
}

// Being able to create shares on the same item does not let a member revoke
// someone else's.
func TestRevokeShare_OtherMemberCannotRevokeWithoutShareRight(t *testing.T) {
	folder, share := sharedFolder(t, "RevokeOthers")

	pr := newRulePolicy()
	pr.grant("ngac-member-2", folder.NgacNodeId, ngac.OpWrite)
	pr.grant("ngac-member-2", folder.NgacNodeId, ngac.OpRead)
	pw := &recordingPolicyWrite{}
	srv, _ := newServerWith(t, pr, pw)

	_, err := srv.RevokeShare(asCaller("", "ngac-member-2"), &pb.RevokeShareRequest{
		ShareId: share.Id,
	})

	assert.Equal(t, codes.PermissionDenied, status.Code(err))
	assert.Empty(t, pw.deleted)
	assert.True(t, shareStillListed(t, folder.Id, share.Id))
}

// A share whose creator was never recorded must not be revocable by a caller
// who is also unidentified: empty does not match empty.
func TestRevokeShare_EmptyCreatorDoesNotMatchEmptyCaller(t *testing.T) {
	allow, pool := setupServer(t)
	wsID := getTestWorkspaceID(t, pool)
	folder, err := allow.CreateFolder(asCaller("", "ngac-owner"), &pb.CreateFolderRequest{
		WorkspaceId: wsID, Name: "RevokeAnon",
	})
	require.NoError(t, err)
	t.Cleanup(func() { cleanDriveItems(t, pool, folder.Id) })
	share, err := allow.CreateShare(context.Background(), &pb.CreateShareRequest{
		ItemId: folder.Id, ShareType: "user", TargetNgacNodeId: "ngac-user-2",
		Operations: []string{ngac.OpRead}, // no caller on the context: created_by is ""
	})
	require.NoError(t, err)

	pw := &recordingPolicyWrite{}
	srv, _ := newServerWith(t, newRulePolicy(), pw)
	_, err = srv.RevokeShare(context.Background(), &pb.RevokeShareRequest{ShareId: share.Id})

	assert.Equal(t, codes.PermissionDenied, status.Code(err))
	assert.Empty(t, pw.deleted)
	assert.True(t, shareStillListed(t, folder.Id, share.Id))
}

// ---------------------------------------------------------------------------
// UpdateQuota — requires manage on the workspace Mgmt OA.
//
// UpdateQuotaRequest carries no caller field, so the caller comes from the
// in-process identity on the context. A call with no identity is denied.
// ---------------------------------------------------------------------------

func quotaLimits(t *testing.T, pool *pgxpool.Pool, wsID string) (int64, int32) {
	t.Helper()
	var maxBytes int64
	var maxFiles int32
	pool.Exec(context.Background(), `INSERT INTO drive_quotas (workspace_id) VALUES ($1) ON CONFLICT DO NOTHING`, wsID)
	require.NoError(t, pool.QueryRow(context.Background(),
		`SELECT max_bytes, max_files FROM drive_quotas WHERE workspace_id = $1`, wsID).Scan(&maxBytes, &maxFiles))
	return maxBytes, maxFiles
}

func restoreQuota(t *testing.T, pool *pgxpool.Pool, wsID string) {
	t.Helper()
	maxBytes, maxFiles := quotaLimits(t, pool, wsID)
	t.Cleanup(func() {
		pool.Exec(context.Background(),
			`UPDATE drive_quotas SET max_bytes = $1, max_files = $2 WHERE workspace_id = $3`, maxBytes, maxFiles, wsID)
	})
}

func TestUpdateQuota_DeniedWithoutManageOnMgmtOA(t *testing.T) {
	pr := newRulePolicy()
	srv, pool := newServerWith(t, pr, &recordingPolicyWrite{})
	wsID := getTestWorkspaceID(t, pool)
	restoreQuota(t, pool, wsID)
	beforeBytes, beforeFiles := quotaLimits(t, pool, wsID)

	// Manage somewhere else, and read on the Mgmt OA, are not enough.
	pr.grant("ngac-member", oaID(ngac.DocumentsOAName(wsID)), ngac.OpManage)
	pr.grant("ngac-member", oaID(ngac.MgmtOAName(wsID)), ngac.OpRead)

	ctx := asCaller("", "ngac-member")
	_, err := srv.UpdateQuota(ctx, &pb.UpdateQuotaRequest{
		WorkspaceId: wsID, MaxBytes: beforeBytes + 12345, MaxFiles: beforeFiles + 7,
	})

	assert.Equal(t, codes.PermissionDenied, status.Code(err))
	afterBytes, afterFiles := quotaLimits(t, pool, wsID)
	assert.Equal(t, beforeBytes, afterBytes, "denied update must not change max_bytes")
	assert.Equal(t, beforeFiles, afterFiles, "denied update must not change max_files")
}

func TestUpdateQuota_DeniedWithoutCaller(t *testing.T) {
	pr := newRulePolicy()
	srv, pool := newServerWith(t, pr, &recordingPolicyWrite{})
	wsID := getTestWorkspaceID(t, pool)
	restoreQuota(t, pool, wsID)
	beforeBytes, _ := quotaLimits(t, pool, wsID)
	// Even a grant to the empty subject must not open the call.
	pr.grant("", oaID(ngac.MgmtOAName(wsID)), ngac.OpManage)

	_, err := srv.UpdateQuota(context.Background(), &pb.UpdateQuotaRequest{
		WorkspaceId: wsID, MaxBytes: beforeBytes + 1, MaxFiles: 1,
	})

	assert.Equal(t, codes.PermissionDenied, status.Code(err))
	afterBytes, _ := quotaLimits(t, pool, wsID)
	assert.Equal(t, beforeBytes, afterBytes)
}

func TestUpdateQuota_DeniedWhenPolicyErrors(t *testing.T) {
	pr := newRulePolicy()
	srv, pool := newServerWith(t, pr, &recordingPolicyWrite{})
	wsID := getTestWorkspaceID(t, pool)
	restoreQuota(t, pool, wsID)
	beforeBytes, _ := quotaLimits(t, pool, wsID)
	pr.grant("ngac-owner", oaID(ngac.MgmtOAName(wsID)), ngac.OpManage)
	pr.failErr = errors.New("policy unavailable")

	ctx := asCaller("", "ngac-owner")
	_, err := srv.UpdateQuota(ctx, &pb.UpdateQuotaRequest{
		WorkspaceId: wsID, MaxBytes: beforeBytes + 1, MaxFiles: 1,
	})

	assert.Equal(t, codes.PermissionDenied, status.Code(err))
	afterBytes, _ := quotaLimits(t, pool, wsID)
	assert.Equal(t, beforeBytes, afterBytes)
}

func TestUpdateQuota_AllowedWithManageOnMgmtOA(t *testing.T) {
	pr := newRulePolicy()
	srv, pool := newServerWith(t, pr, &recordingPolicyWrite{})
	wsID := getTestWorkspaceID(t, pool)
	restoreQuota(t, pool, wsID)
	pr.grant("ngac-owner", oaID(ngac.MgmtOAName(wsID)), ngac.OpManage)

	// The response re-reads the quota, which takes read on the drive root.
	pr.grant("ngac-owner", anyObject, ngac.OpRead)

	ctx := asCaller("", "ngac-owner")
	q, err := srv.UpdateQuota(ctx, &pb.UpdateQuotaRequest{
		WorkspaceId: wsID, MaxBytes: 987654321, MaxFiles: 4321,
	})

	require.NoError(t, err)
	assert.Contains(t, pr.checks, [3]string{"ngac-owner", oaID(ngac.MgmtOAName(wsID)), ngac.OpManage})
	assert.Equal(t, int64(987654321), q.MaxBytes)
	assert.Equal(t, int32(4321), q.MaxFiles)
}

// asCaller returns a context carrying the caller the interceptor would have
// put there from request metadata.
func asCaller(userID, nodeID string) context.Context {
	return grpcauth.WithCaller(context.Background(), grpcauth.Caller{UserID: userID, NGACNodeID: nodeID})
}
