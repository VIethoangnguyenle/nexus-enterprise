package grpc_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"ngac-platform/ngac"
	docpb "ngac-platform/proto/document"
	pb "ngac-platform/proto/drive"
	policypb "ngac-platform/proto/policy"
)

// policyWithBrokenGraphReads answers access checks from rules but fails the
// graph lookups the way an unreachable policy service would.
type policyWithBrokenGraphReads struct {
	*rulePolicyRead
	failPCGlobal  bool
	failAncestors bool
}

func (p *policyWithBrokenGraphReads) FindNodeByName(ctx context.Context, req *policypb.FindNodeByNameRequest, o ...grpc.CallOption) (*policypb.NGACNode, error) {
	if p.failPCGlobal && req.Name == ngac.NodePCGlobal {
		return nil, status.Error(codes.Unavailable, "policy service down")
	}
	return p.rulePolicyRead.FindNodeByName(ctx, req, o...)
}

func (p *policyWithBrokenGraphReads) GetAncestors(ctx context.Context, req *policypb.GetAncestorsRequest, o ...grpc.CallOption) (*policypb.NodeList, error) {
	if p.failAncestors {
		return nil, status.Error(codes.Unavailable, "policy service down")
	}
	return p.rulePolicyRead.GetAncestors(ctx, req, o...)
}

// A share whose policy class cannot be looked up must not be created half way:
// the share OA made for it is removed and no row is left to show a share that
// grants nothing.
func TestCreateShare_ALookupFailureLeavesNothingBehind(t *testing.T) {
	folder, _ := sharedFolder(t, "ShareHalfWay")
	pr := newRulePolicy()
	pr.grant("ngac-owner", anyObject, ngac.OpShare)
	pw := &recordingPolicyWrite{}
	srv, _ := newServerWith(t, &policyWithBrokenGraphReads{rulePolicyRead: pr, failPCGlobal: true}, pw)

	_, err := srv.CreateShare(asCaller("", "ngac-owner"), &pb.CreateShareRequest{
		ItemId: folder.Id, ShareType: "user", TargetNgacNodeId: "ngac-user-3",
		Operations: []string{ngac.OpRead},
	})

	require.Error(t, err)
	assert.Equal(t, codes.Internal, status.Code(err))
	assert.NotContains(t, status.Convert(err).Message(), "policy service down", "the cause stays in the log")
	assert.NotEmpty(t, pw.deleted, "the share OA created before the failure was removed")

	allow, _ := setupServer(t)
	list, lerr := allow.ListShares(asCaller("", "ngac-owner"), &pb.ListSharesRequest{ItemId: folder.Id})
	require.NoError(t, lerr)
	for _, sh := range list.Shares {
		assert.NotEqual(t, "ngac-user-3", sh.TargetNgacId, "no share row for the failed share")
	}
}

// An unreachable policy service must not turn "shared with me" into a shorter
// list that omits everything shared with the caller's groups.
func TestGetSharedWithMe_APolicyFailureIsAnErrorNotAShorterList(t *testing.T) {
	srv, _ := newServerWith(t, &policyWithBrokenGraphReads{rulePolicyRead: newRulePolicy(), failAncestors: true}, &recordingPolicyWrite{})

	_, err := srv.GetSharedWithMe(asCaller("u", "ngac-user-2"), &pb.GetSharedWithMeRequest{})

	require.Error(t, err)
	assert.Equal(t, codes.Internal, status.Code(err))
}

// Confirming a file charges the quota once, however many times the client asks.
func TestConfirmFile_ASecondConfirmIsRefusedAndChargesNothing(t *testing.T) {
	srv, pool := setupServer(t)
	wsID := getTestWorkspaceID(t, pool)
	userID := getTestUserID(t, pool)
	ctx := asCaller(userID, "ngac-user-1")

	created, err := srv.CreateFile(ctx, &pb.CreateFileRequest{WorkspaceId: wsID, Name: "twice.pdf", MimeType: "application/pdf", SizeBytes: 1024})
	require.NoError(t, err)
	t.Cleanup(func() { cleanDriveItems(t, pool, created.FileId) })
	before, err := srv.GetQuota(ctx, &pb.GetQuotaRequest{WorkspaceId: wsID})
	require.NoError(t, err)

	_, err = srv.ConfirmFile(ctx, &pb.ConfirmFileRequest{FileId: created.FileId})
	require.NoError(t, err)
	_, err = srv.ConfirmFile(ctx, &pb.ConfirmFileRequest{FileId: created.FileId})

	assert.Equal(t, codes.FailedPrecondition, status.Code(err))
	after, err := srv.GetQuota(ctx, &pb.GetQuotaRequest{WorkspaceId: wsID})
	require.NoError(t, err)
	assert.Equal(t, before.UsedFiles+1, after.UsedFiles, "charged once")
	assert.Equal(t, before.UsedBytes+1024, after.UsedBytes)
}

// docFailing makes the document service answer ConfirmUpload with err.
type docFailing struct {
	*mockDocStorage
	err error
}

func (d docFailing) ConfirmUpload(context.Context, *docpb.ConfirmUploadRequest, ...grpc.CallOption) (*docpb.ConfirmUploadResponse, error) {
	return nil, d.err
}

// A file the store never received is a 409 with a fixed message; a document
// service that is down is our failure, and nothing of its error reaches the caller.
func TestConfirmFile_StorageTextNeverReachesTheCaller(t *testing.T) {
	_, pool := setupServer(t)
	wsID := getTestWorkspaceID(t, pool)
	userID := getTestUserID(t, pool)
	ctx := asCaller(userID, "ngac-user-1")
	setup := newDrive(pool, &mockPolicyRead{}, &mockPolicyWrite{}, &mockDocStorage{})
	created, err := setup.CreateFile(ctx, &pb.CreateFileRequest{WorkspaceId: wsID, Name: "x.pdf", MimeType: "application/pdf", SizeBytes: 10})
	require.NoError(t, err)
	t.Cleanup(func() { cleanDriveItems(t, pool, created.FileId) })

	missing := newDrive(pool, &mockPolicyRead{}, &mockPolicyWrite{}, docFailing{mockDocStorage: &mockDocStorage{}, err: status.Error(codes.FailedPrecondition, "file not uploaded: dial tcp 10.0.0.5:9000")})
	_, err = missing.ConfirmFile(ctx, &pb.ConfirmFileRequest{FileId: created.FileId})
	assert.Equal(t, codes.FailedPrecondition, status.Code(err))
	assert.Equal(t, "file not uploaded", status.Convert(err).Message())

	down := newDrive(pool, &mockPolicyRead{}, &mockPolicyWrite{}, docFailing{mockDocStorage: &mockDocStorage{}, err: status.Error(codes.Unavailable, "dial tcp 10.0.0.5:9000: refused")})
	_, err = down.ConfirmFile(ctx, &pb.ConfirmFileRequest{FileId: created.FileId})
	assert.Equal(t, codes.Internal, status.Code(err))
	assert.NotContains(t, status.Convert(err).Message(), "10.0.0.5")
}
