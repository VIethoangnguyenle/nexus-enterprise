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
	"ngac-platform/pkg/grpcauth"
	pb "ngac-platform/proto/drive"
	"ngac-platform/testutil"
)

func serveDrive(t *testing.T, pr *rulePolicyRead) pb.DriveServiceClient {
	t.Helper()
	srv, _ := newServerWith(t, pr, &recordingPolicyWrite{})
	conn := testutil.ServeGRPC(t, grpcauth.ServerPolicy{}, func(s *grpc.Server) {
		pb.RegisterDriveServiceServer(s, srv)
	})
	return pb.NewDriveServiceClient(conn)
}

func TestDriveOverTheWire_MissingCallerIsUnauthenticated(t *testing.T) {
	folder, _ := sharedFolder(t, "WireNoCaller")
	c := serveDrive(t, newRulePolicy())

	_, err := c.GetItem(context.Background(), &pb.GetItemRequest{ItemId: folder.Id})
	assert.Equal(t, codes.Unauthenticated, status.Code(err))

	_, err = c.UpdateQuota(context.Background(), &pb.UpdateQuotaRequest{WorkspaceId: "ws"})
	assert.Equal(t, codes.Unauthenticated, status.Code(err))
}

// Body names a user who may read; the metadata names one who may not.
func TestDriveOverTheWire_BodyCallerIsIgnored_Deny(t *testing.T) {
	folder, _ := sharedFolder(t, "WireDeny")
	pr := newRulePolicy()
	pr.grant("ngac-reader", folder.NgacNodeId, ngac.OpRead)
	c := serveDrive(t, pr)

	_, err := c.GetItem(asCaller("u-other", "ngac-other"), &pb.GetItemRequest{
		ItemId:         folder.Id,
		UserNgacNodeId: "ngac-reader", //lint:ignore SA1019 proves the deprecated body field is not trusted
	})

	assert.Equal(t, codes.PermissionDenied, status.Code(err))
	for _, k := range pr.checks {
		assert.NotEqual(t, "ngac-reader", k[0], "the body's user must never reach the policy check")
	}
}

func TestDriveOverTheWire_MetadataCallerDecides_Allow(t *testing.T) {
	folder, _ := sharedFolder(t, "WireAllow")
	pr := newRulePolicy()
	pr.grant("ngac-reader", folder.NgacNodeId, ngac.OpRead)
	c := serveDrive(t, pr)

	got, err := c.GetItem(asCaller("u-reader", "ngac-reader"), &pb.GetItemRequest{
		ItemId:         folder.Id,
		UserNgacNodeId: "ngac-other", //lint:ignore SA1019 proves the deprecated body field is not trusted
	})

	require.NoError(t, err)
	assert.Equal(t, folder.Id, got.Id)
}

// UpdateQuota has no caller in its request; it used to be refused for every
// caller on the network.
func TestDriveOverTheWire_UpdateQuotaUsesMetadataCaller(t *testing.T) {
	_, pool := newServerWith(t, newRulePolicy(), &recordingPolicyWrite{})
	wsID := getTestWorkspaceID(t, pool)
	restoreQuota(t, pool, wsID)
	beforeBytes, beforeFiles := quotaLimits(t, pool, wsID)

	pr := newRulePolicy()
	pr.grant("ngac-admin", oaID(ngac.MgmtOAName(wsID)), ngac.OpManage)
	pr.grant("ngac-admin", anyObject, ngac.OpRead) // GetQuota, which UpdateQuota returns, needs read
	c := serveDrive(t, pr)

	_, err := c.UpdateQuota(asCaller("u-other", "ngac-other"), &pb.UpdateQuotaRequest{WorkspaceId: wsID, MaxBytes: beforeBytes + 1, MaxFiles: beforeFiles})
	assert.Equal(t, codes.PermissionDenied, status.Code(err))

	_, err = c.UpdateQuota(asCaller("u-admin", "ngac-admin"), &pb.UpdateQuotaRequest{WorkspaceId: wsID, MaxBytes: beforeBytes + 1, MaxFiles: beforeFiles})
	require.NoError(t, err)
}
