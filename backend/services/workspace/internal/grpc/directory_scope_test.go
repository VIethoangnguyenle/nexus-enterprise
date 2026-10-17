package grpc_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"ngac-platform/pkg/grpcauth"
	"ngac-platform/pkg/httputil"
	pb "ngac-platform/proto/workspace"
	"ngac-platform/services/workspace/internal/domain"
	grpcserver "ngac-platform/services/workspace/internal/grpc"
	"ngac-platform/testutil"
)

// GetDirectoryScope is what the auth service asks, on behalf of the person whose request it is
// serving, before it filters the contacts it lists. The person is the verified caller on the
// call, never a field of the body; a service speaking only for itself has no scope to ask for.

type scopeStub struct {
	grpcserver.WorkspaceDomainService // every other method panics: the test reaches none
	scope                             *domain.DirectoryScope
	err                               error
	callerNode, ws                    string
	calls                             int
}

func (s *scopeStub) DirectoryScope(_ context.Context, caller, ws string) (*domain.DirectoryScope, error) {
	s.calls++
	s.callerNode, s.ws = caller, ws
	return s.scope, s.err
}

func serveScope(t *testing.T, stub *scopeStub, service string) pb.WorkspaceServiceClient {
	t.Helper()
	srv := grpcserver.NewWorkspaceServer(stub)
	conn := testutil.ServeGRPCAs(t, service, grpcauth.ServerPolicy{Exempt: grpcauth.HealthExempt()}, func(s *grpc.Server) { pb.RegisterWorkspaceServiceServer(s, srv) })
	return pb.NewWorkspaceServiceClient(conn)
}

func TestGetDirectoryScope_ServesTheForwardedCallerOnly(t *testing.T) {
	stub := &scopeStub{scope: &domain.DirectoryScope{
		Ready: true, Limited: true,
		VisibleDeptIDs: map[string]bool{"d2": true, "d1": true, "d3": false},
		OwnersVisible:  true, OwnerNodeIDs: []string{"n-owner"},
		HiddenDeptIDs: map[string]bool{"d9": true},
		Roles: []domain.RoleHolders{
			{ID: "ua-owners", Kind: domain.RoleOwners, HolderNodeIDs: []string{"n-owner"}},
			{ID: "ua-r", Name: "Kế toán", Kind: domain.RoleCustom, HolderNodeIDs: []string{"n-1", "n-2"}},
		},
	}}

	// DENY: a service on its own identity, any service, has no person to answer for.
	for _, service := range []string{"auth", "messaging", "workspace", "calendar"} {
		c := serveScope(t, stub, service)
		_, err := c.GetDirectoryScope(context.Background(), &pb.GetDirectoryScopeRequest{WorkspaceId: "ws-1"})
		assert.Equal(t, codes.Unauthenticated, status.Code(err), service)
	}
	assert.Zero(t, stub.calls, "the domain was never reached without a person")

	// ALLOW on the same stub: the auth service speaking as the person.
	c := serveScope(t, stub, "auth")
	got, err := c.GetDirectoryScope(asCaller("user-1", "n-caller"), &pb.GetDirectoryScopeRequest{WorkspaceId: "ws-1"})
	require.NoError(t, err)
	assert.Equal(t, "n-caller", stub.callerNode, "the caller is the one on the wire")
	assert.Equal(t, "ws-1", stub.ws)
	assert.True(t, got.Ready)
	assert.True(t, got.Limited)
	assert.False(t, got.Admin)
	assert.Equal(t, []string{"d1", "d2"}, got.VisibleDepartmentIds, "only the departments that are visible, in order")
	assert.True(t, got.OwnersVisible)
	assert.Equal(t, []string{"n-owner"}, got.OwnerNodeIds)
	assert.Equal(t, []string{"d9"}, got.HiddenDepartmentIds)
	require.Len(t, got.Roles, 2)
	assert.Equal(t, "owners", got.Roles[0].Kind)
	assert.Equal(t, "Kế toán", got.Roles[1].Name)
	assert.Equal(t, []string{"n-1", "n-2"}, got.Roles[1].HolderNodeIds)
}

func TestGetDirectoryScope_ErrorsKeepTheirMeaning(t *testing.T) {
	for name, tc := range map[string]struct {
		err  error
		code codes.Code
	}{
		"not a member":  {fmt.Errorf("%w: not a member", httputil.ErrAccessDenied), codes.PermissionDenied},
		"unknown":       {fmt.Errorf("%w: workspace", httputil.ErrNotFound), codes.NotFound},
		"policy outage": {fmt.Errorf("%w: policy", httputil.ErrUnavailable), codes.Unavailable},
		"anything else": {fmt.Errorf("boom: SQLSTATE 42"), codes.Internal},
	} {
		stub := &scopeStub{err: tc.err}
		c := serveScope(t, stub, "auth")
		_, err := c.GetDirectoryScope(asCaller("user-1", "n-caller"), &pb.GetDirectoryScopeRequest{WorkspaceId: "ws-1"})
		assert.Equal(t, tc.code, status.Code(err), name)
		assert.NotContains(t, status.Convert(err).Message(), "SQLSTATE", name)
	}

	stub := &scopeStub{}
	c := serveScope(t, stub, "auth")
	_, err := c.GetDirectoryScope(asCaller("user-1", "n-caller"), &pb.GetDirectoryScopeRequest{})
	assert.Equal(t, codes.InvalidArgument, status.Code(err), "a workspace is required")
	assert.Zero(t, stub.calls)
}

// recorder (adapter_test.go) satisfies the domain interface; its scope is an empty one.
func (r *recorder) DirectoryScope(_ context.Context, c, ws string) (*domain.DirectoryScope, error) {
	return &domain.DirectoryScope{}, r.note(c, ws)
}
