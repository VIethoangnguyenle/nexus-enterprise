package grpc_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"ngac-platform/pkg/grpcauth"
	pb "ngac-platform/proto/workspace"
	"ngac-platform/testutil"
)

// DeleteWorkspace is for compensation and must never run for nobody: the caller
// is taken from the verified request metadata, and without one the call is
// refused before it reaches the domain.
func TestDeleteWorkspace_NeedsACaller(t *testing.T) {
	srv, _, _ := setupTestServer(t)
	conn := testutil.ServeGRPC(t, grpcauth.ServerPolicy{}, func(s *grpc.Server) { pb.RegisterWorkspaceServiceServer(s, srv) })
	c := pb.NewWorkspaceServiceClient(conn)

	_, err := c.DeleteWorkspace(context.Background(), &pb.DeleteWorkspaceRequest{WorkspaceId: "any"})

	assert.Equal(t, codes.Unauthenticated, status.Code(err))
}

// The body names the workspace and nothing about who is asking.
func TestDeleteWorkspace_RequestCarriesOnlyTheWorkspace(t *testing.T) {
	fields := (&pb.DeleteWorkspaceRequest{}).ProtoReflect().Descriptor().Fields()
	assert.Equal(t, 1, fields.Len())
	assert.Equal(t, "workspace_id", string(fields.Get(0).Name()))
}
