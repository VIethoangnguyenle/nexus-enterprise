package grpc_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"ngac-platform/pkg/grpcauth"
	pb "ngac-platform/proto/workspace"
	"ngac-platform/services/workspace/internal/domain"
	grpcserver "ngac-platform/services/workspace/internal/grpc"
)

// recorder is the domain as the adapter sees it: it remembers who asked and
// what was asked, and fails with err when set.
type recorder struct {
	err    error
	caller string
	args   []string
}

func (r *recorder) note(caller string, args ...string) error {
	r.caller, r.args = caller, args
	return r.err
}

func (r *recorder) CreateWorkspace(_ context.Context, in domain.CreateWorkspaceInput) (*domain.WorkspaceResult, error) {
	return &domain.WorkspaceResult{ID: "ws", Name: in.Name}, r.note(in.UserNGACNodeID, in.Name)
}
func (r *recorder) ViewWorkspace(_ context.Context, c, id string) (*domain.WorkspaceResult, error) {
	return &domain.WorkspaceResult{ID: id}, r.note(c, id)
}
func (r *recorder) ListAccessibleWorkspaces(_ context.Context, c string) ([]*domain.WorkspaceResult, error) {
	return []*domain.WorkspaceResult{{ID: "a"}, {ID: "b"}}, r.note(c)
}
func (r *recorder) RemoveMember(_ context.Context, c, ws, target string) error {
	return r.note(c, ws, target)
}
func (r *recorder) ListMembers(_ context.Context, c, ws string) ([]*domain.Member, error) {
	return []*domain.Member{{NGACNodeID: "m"}}, r.note(c, ws)
}
func (r *recorder) TransferOwnership(_ context.Context, c, ws, n string) error {
	return r.note(c, ws, n)
}
func (r *recorder) RemoveOwner(_ context.Context, c, ws, target string) error {
	return r.note(c, ws, target)
}
func (r *recorder) CreateRole(_ context.Context, c, ws, name string) (*domain.Role, error) {
	return &domain.Role{ID: "r", Name: name}, r.note(c, ws, name)
}
func (r *recorder) ListRoles(_ context.Context, c, ws string) ([]*domain.Role, error) {
	return []*domain.Role{{ID: "r", Name: "n"}}, r.note(c, ws)
}
func (r *recorder) DeleteRole(_ context.Context, c, ws, id string) error { return r.note(c, ws, id) }
func (r *recorder) CreateFolder(_ context.Context, c, ws, name, parent string) (*domain.Folder, error) {
	return &domain.Folder{ID: "f", Name: name}, r.note(c, ws, name, parent)
}
func (r *recorder) ListFolders(_ context.Context, c, ws string) ([]*domain.Folder, error) {
	return []*domain.Folder{{ID: "f", Name: "n"}}, r.note(c, ws)
}
func (r *recorder) DeleteFolder(_ context.Context, c, ws, id string) error { return r.note(c, ws, id) }

// asCaller is a request context carrying the verified caller.
func callerCtx(node string) context.Context {
	return grpcauth.WithCaller(context.Background(), grpcauth.Caller{UserID: "user-" + node, NGACNodeID: node})
}

// Every handler passes the caller from the verified context, never from the
// body, and hands the domain exactly the ids the request named.
func TestAdapter_CallerComesFromTheContextAndArgumentsPassThrough(t *testing.T) {
	r := &recorder{}
	s := grpcserver.NewWorkspaceServer(r)
	ctx := callerCtx("node-alice")

	for name, tc := range map[string]struct {
		call func() error
		want []string
	}{
		"get":  {func() error { _, e := s.GetWorkspace(ctx, &pb.GetWorkspaceRequest{WorkspaceId: "ws1"}); return e }, []string{"ws1"}},
		"list": {func() error { _, e := s.ListWorkspaces(ctx, &pb.ListWorkspacesRequest{}); return e }, nil},
		"remove member": {func() error {
			_, e := s.RemoveMember(ctx, &pb.RemoveMemberRequest{WorkspaceId: "ws1", TargetNgacNodeId: "bob"})
			return e
		}, []string{"ws1", "bob"}},
		"list members": {func() error { _, e := s.ListMembers(ctx, &pb.ListMembersRequest{WorkspaceId: "ws1"}); return e }, []string{"ws1"}},
		"transfer": {func() error {
			_, e := s.TransferOwnership(ctx, &pb.TransferOwnershipRequest{WorkspaceId: "ws1", NewOwnerNgacNodeId: "bob"})
			return e
		}, []string{"ws1", "bob"}},
		"add owner": {func() error {
			_, e := s.AddOwner(ctx, &pb.AddOwnerRequest{WorkspaceId: "ws1", TargetNgacNodeId: "bob"})
			return e
		}, []string{"ws1", "bob"}},
		"remove owner": {func() error {
			_, e := s.RemoveOwner(ctx, &pb.RemoveOwnerRequest{WorkspaceId: "ws1", TargetNgacNodeId: "bob"})
			return e
		}, []string{"ws1", "bob"}},
		"create role": {func() error {
			_, e := s.CreateRole(ctx, &pb.CreateRoleRequest{WorkspaceId: "ws1", Name: "Chief"})
			return e
		}, []string{"ws1", "Chief"}},
		"list roles": {func() error { _, e := s.ListRoles(ctx, &pb.ListRolesRequest{WorkspaceId: "ws1"}); return e }, []string{"ws1"}},
		"delete role": {func() error {
			_, e := s.DeleteRole(ctx, &pb.DeleteRoleRequest{WorkspaceId: "ws1", RoleId: "r9"})
			return e
		}, []string{"ws1", "r9"}},
		"create folder": {func() error {
			_, e := s.CreateFolder(ctx, &pb.CreateFolderRequest{WorkspaceId: "ws1", Name: "Docs", ParentOaId: "p1"})
			return e
		}, []string{"ws1", "Docs", "p1"}},
		"list folders": {func() error { _, e := s.ListFolders(ctx, &pb.ListFoldersRequest{WorkspaceId: "ws1"}); return e }, []string{"ws1"}},
		"delete folder": {func() error {
			_, e := s.DeleteFolder(ctx, &pb.DeleteFolderRequest{WorkspaceId: "ws1", FolderId: "f9"})
			return e
		}, []string{"ws1", "f9"}},
	} {
		*r = recorder{}
		require.NoError(t, tc.call(), name)
		assert.Equal(t, "node-alice", r.caller, "%s: the caller is the verified one", name)
		assert.Equal(t, tc.want, r.args, name)
	}
}

// With no verified caller the domain is still asked, with an empty caller, and
// the domain is where an empty caller is denied.
func TestAdapter_NoCallerReachesTheDomainAsEmpty(t *testing.T) {
	r := &recorder{err: domain.ErrAccessDenied}
	s := grpcserver.NewWorkspaceServer(r)

	_, err := s.RemoveMember(context.Background(), &pb.RemoveMemberRequest{WorkspaceId: "ws1", TargetNgacNodeId: "bob"})

	assert.Equal(t, codes.PermissionDenied, status.Code(err))
	assert.Equal(t, "", r.caller)
}

func TestAdapter_DomainFailuresBecomeTheRightStatus(t *testing.T) {
	ctx := callerCtx("node-alice")
	for name, tc := range map[string]struct {
		err  error
		want codes.Code
	}{
		"denied":   {domain.ErrAccessDenied, codes.PermissionDenied},
		"missing":  {domain.ErrNotFound, codes.NotFound},
		"invalid":  {domain.ErrInvalidInput, codes.InvalidArgument},
		"exists":   {domain.ErrAlreadyExists, codes.AlreadyExists},
		"database": {errors.New(`load members: ERROR: relation "x" does not exist (SQLSTATE 42P01)`), codes.Internal},
	} {
		s := grpcserver.NewWorkspaceServer(&recorder{err: tc.err})
		calls := map[string]func() error{
			"remove member": func() error { _, e := s.RemoveMember(ctx, &pb.RemoveMemberRequest{WorkspaceId: "w"}); return e },
			"transfer": func() error {
				_, e := s.TransferOwnership(ctx, &pb.TransferOwnershipRequest{WorkspaceId: "w"})
				return e
			},
			"remove owner":    func() error { _, e := s.RemoveOwner(ctx, &pb.RemoveOwnerRequest{WorkspaceId: "w"}); return e },
			"delete role":     func() error { _, e := s.DeleteRole(ctx, &pb.DeleteRoleRequest{WorkspaceId: "w"}); return e },
			"delete folder":   func() error { _, e := s.DeleteFolder(ctx, &pb.DeleteFolderRequest{WorkspaceId: "w"}); return e },
			"list workspaces": func() error { _, e := s.ListWorkspaces(ctx, &pb.ListWorkspacesRequest{}); return e },
			"list folders":    func() error { _, e := s.ListFolders(ctx, &pb.ListFoldersRequest{WorkspaceId: "w"}); return e },
		}
		for op, call := range calls {
			err := call()
			assert.Equal(t, tc.want, status.Code(err), "%s / %s", name, op)
			if tc.want == codes.Internal {
				assert.NotContains(t, status.Convert(err).Message(), "SQLSTATE", "%s: the cause stays out of the message", op)
			}
		}
	}
}

func TestAdapter_ListsConvertEveryEntry(t *testing.T) {
	s := grpcserver.NewWorkspaceServer(&recorder{})
	ctx := callerCtx("node-alice")

	ws, err := s.ListWorkspaces(ctx, &pb.ListWorkspacesRequest{})
	require.NoError(t, err)
	assert.Len(t, ws.Workspaces, 2)
	folders, err := s.ListFolders(ctx, &pb.ListFoldersRequest{WorkspaceId: "w"})
	require.NoError(t, err)
	assert.Len(t, folders.Folders, 1)
}

func TestAdapter_CreatesNeedANameBeforeTheDomainIsAsked(t *testing.T) {
	r := &recorder{}
	s := grpcserver.NewWorkspaceServer(r)
	ctx := callerCtx("node-alice")

	_, err := s.CreateWorkspace(ctx, &pb.CreateWorkspaceRequest{})
	assert.Equal(t, codes.InvalidArgument, status.Code(err))
	_, err = s.CreateRole(ctx, &pb.CreateRoleRequest{WorkspaceId: "w"})
	assert.Equal(t, codes.InvalidArgument, status.Code(err))
	_, err = s.CreateFolder(ctx, &pb.CreateFolderRequest{WorkspaceId: "w"})
	assert.Equal(t, codes.InvalidArgument, status.Code(err))
	assert.Empty(t, r.args, "nothing reached the domain")
}

// DeleteWorkspace asks the domain with the verified caller's node and user.
func TestDeleteWorkspace_UsesTheVerifiedCaller(t *testing.T) {
	d := &deleter{recorder: recorder{}}
	s := grpcserver.NewWorkspaceServer(d)

	_, err := s.DeleteWorkspace(callerCtx("node-alice"), &pb.DeleteWorkspaceRequest{WorkspaceId: "ws1"})
	require.NoError(t, err)
	assert.Equal(t, []string{"node-alice", "user-node-alice", "ws1"}, d.got)

	d.err = domain.ErrAccessDenied
	_, err = s.DeleteWorkspace(callerCtx("node-alice"), &pb.DeleteWorkspaceRequest{WorkspaceId: "ws1"})
	assert.Equal(t, codes.PermissionDenied, status.Code(err))

	// A domain without the operation answers Unimplemented, never success.
	_, err = grpcserver.NewWorkspaceServer(&recorder{}).DeleteWorkspace(callerCtx("n"), &pb.DeleteWorkspaceRequest{WorkspaceId: "ws1"})
	assert.Equal(t, codes.Unimplemented, status.Code(err))
}

type deleter struct {
	recorder
	got []string
}

func (d *deleter) DeleteWorkspace(_ context.Context, node, user, ws string) error {
	d.got = []string{node, user, ws}
	return d.err
}
