package rest

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"ngac-platform/pkg/httputil"
	pb "ngac-platform/proto/workspace"
	"ngac-platform/services/workspace/internal/domain"
)

const callerNode = "u-caller"

// fakeWorkspaceService records what each handler passes down and returns err.
type fakeWorkspaceService struct {
	err       error
	requester string // requester seen by the last call (field or context)
	calls     int
}

func (f *fakeWorkspaceService) seen(ctx context.Context, field string) error {
	f.calls++
	f.requester = field
	if field == "" {
		f.requester = domain.RequesterFrom(ctx)
	}
	return f.err
}

func (f *fakeWorkspaceService) CreateWorkspace(ctx context.Context, req *pb.CreateWorkspaceRequest) (*pb.Workspace, error) {
	return &pb.Workspace{}, f.seen(ctx, req.UserNgacNodeId)
}
func (f *fakeWorkspaceService) ListWorkspaces(ctx context.Context, req *pb.ListWorkspacesRequest) (*pb.WorkspaceList, error) {
	return &pb.WorkspaceList{}, f.seen(ctx, req.UserNgacNodeId)
}
func (f *fakeWorkspaceService) GetWorkspace(ctx context.Context, _ *pb.GetWorkspaceRequest) (*pb.Workspace, error) {
	return &pb.Workspace{}, f.seen(ctx, "")
}
func (f *fakeWorkspaceService) InviteMember(ctx context.Context, req *pb.InviteMemberRequest) (*pb.Empty, error) {
	return &pb.Empty{}, f.seen(ctx, req.InviterNgacNodeId)
}
func (f *fakeWorkspaceService) RemoveMember(ctx context.Context, req *pb.RemoveMemberRequest) (*pb.Empty, error) {
	return &pb.Empty{}, f.seen(ctx, req.RequesterNgacNodeId)
}
func (f *fakeWorkspaceService) ListMembers(ctx context.Context, _ *pb.ListMembersRequest) (*pb.MemberList, error) {
	return &pb.MemberList{}, f.seen(ctx, "")
}
func (f *fakeWorkspaceService) CreateRole(ctx context.Context, req *pb.CreateRoleRequest) (*pb.Role, error) {
	return &pb.Role{}, f.seen(ctx, req.RequesterNgacNodeId)
}
func (f *fakeWorkspaceService) ListRoles(ctx context.Context, _ *pb.ListRolesRequest) (*pb.RoleList, error) {
	return &pb.RoleList{}, f.seen(ctx, "")
}
func (f *fakeWorkspaceService) CreateFolder(ctx context.Context, req *pb.CreateFolderRequest) (*pb.Folder, error) {
	return &pb.Folder{}, f.seen(ctx, req.RequesterNgacNodeId)
}
func (f *fakeWorkspaceService) CreatePermission(ctx context.Context, req *pb.CreatePermissionRequest) (*pb.Permission, error) {
	return &pb.Permission{}, f.seen(ctx, req.RequesterNgacNodeId)
}

type route struct {
	name    string
	method  string
	body    string
	params  []string
	handler func(h *Handler) echo.HandlerFunc
}

// Bodies deliberately carry spoofed requester fields: the handler must ignore
// them and take the caller from the verified JWT claims only.
const spoof = `"requester_ngac_node_id":"u-attacker","inviter_ngac_node_id":"u-attacker"`

func guardedRoutes() []route {
	return []route{
		{"GetWorkspace", http.MethodGet, "", []string{"id"}, func(h *Handler) echo.HandlerFunc { return h.GetWorkspace }},
		{"InviteMember", http.MethodPost, `{"ngac_node_id":"u-target",` + spoof + `}`, []string{"id"}, func(h *Handler) echo.HandlerFunc { return h.InviteMember }},
		{"RemoveMember", http.MethodDelete, "", []string{"id", "nodeId"}, func(h *Handler) echo.HandlerFunc { return h.RemoveMember }},
		{"ListMembers", http.MethodGet, "", []string{"id"}, func(h *Handler) echo.HandlerFunc { return h.ListMembers }},
		{"CreateRole", http.MethodPost, `{"name":"Editor",` + spoof + `}`, []string{"id"}, func(h *Handler) echo.HandlerFunc { return h.CreateRole }},
		{"ListRoles", http.MethodGet, "", []string{"id"}, func(h *Handler) echo.HandlerFunc { return h.ListRoles }},
		{"CreateFolder", http.MethodPost, `{"name":"Legal",` + spoof + `}`, []string{"id"}, func(h *Handler) echo.HandlerFunc { return h.CreateFolder }},
		{"CreatePermission", http.MethodPost, `{"ua_id":"ua","oa_id":"oa","operations":["read"],` + spoof + `}`, []string{"id"}, func(h *Handler) echo.HandlerFunc { return h.CreatePermission }},
	}
}

func newCtx(t *testing.T, r route, withClaims bool) (echo.Context, *httptest.ResponseRecorder) {
	t.Helper()
	e := echo.New()
	req := httptest.NewRequest(r.method, "/api/workspaces/ws-1", strings.NewReader(r.body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames(r.params...)
	vals := []string{"ws-1", "u-target"}
	c.SetParamValues(vals[:len(r.params)]...)
	if withClaims {
		httputil.SetClaims(c, &httputil.Claims{UserID: "user-1", NGACNodeID: callerNode})
	}
	return c, rec
}

func TestHandler_CallerComesFromClaims(t *testing.T) {
	for _, r := range guardedRoutes() {
		t.Run(r.name, func(t *testing.T) {
			f := &fakeWorkspaceService{}
			c, _ := newCtx(t, r, true)
			require.NoError(t, r.handler(NewHandler(f))(c))
			assert.Equal(t, callerNode, f.requester)
		})
	}
}

func TestHandler_PermissionDeniedIs403(t *testing.T) {
	for _, r := range guardedRoutes() {
		t.Run(r.name, func(t *testing.T) {
			f := &fakeWorkspaceService{err: status.Error(codes.PermissionDenied, "access denied: manage")}
			c, _ := newCtx(t, r, true)
			err := r.handler(NewHandler(f))(c)
			var he *echo.HTTPError
			require.True(t, errors.As(err, &he), "err = %v", err)
			assert.Equal(t, http.StatusForbidden, he.Code)
		})
	}
}

func TestHandler_MissingClaimsIs401(t *testing.T) {
	for _, r := range guardedRoutes() {
		t.Run(r.name, func(t *testing.T) {
			f := &fakeWorkspaceService{}
			c, _ := newCtx(t, r, false)
			err := r.handler(NewHandler(f))(c)
			var he *echo.HTTPError
			require.True(t, errors.As(err, &he), "err = %v", err)
			assert.Equal(t, http.StatusUnauthorized, he.Code)
			assert.Zero(t, f.calls, "service must not be reached without claims")
		})
	}
}

// ---------------------------------------------------------------------------
// Admin (department) routes
// ---------------------------------------------------------------------------

type fakeDeptService struct {
	err    error
	caller string
	wsID   string
	calls  int
}

func (f *fakeDeptService) rec(caller, wsID string) error {
	f.calls++
	f.caller, f.wsID = caller, wsID
	return f.err
}

func (f *fakeDeptService) CreateDepartment(_ context.Context, caller string, in domain.CreateDepartmentInput) (*domain.DepartmentResult, error) {
	return &domain.DepartmentResult{}, f.rec(caller, in.WorkspaceID)
}
func (f *fakeDeptService) ListDepartments(_ context.Context, caller, wsID string) ([]*domain.DepartmentResult, error) {
	return nil, f.rec(caller, wsID)
}
func (f *fakeDeptService) UpdateDepartment(_ context.Context, caller, wsID, _, _ string) (*domain.DepartmentResult, error) {
	return &domain.DepartmentResult{}, f.rec(caller, wsID)
}
func (f *fakeDeptService) DeleteDepartment(_ context.Context, caller, wsID, _ string) error {
	return f.rec(caller, wsID)
}
func (f *fakeDeptService) MoveDepartment(_ context.Context, caller string, in domain.MoveDepartmentInput) (*domain.DepartmentResult, error) {
	return &domain.DepartmentResult{}, f.rec(caller, in.WorkspaceID)
}
func (f *fakeDeptService) UpdateMemberDepartment(_ context.Context, caller, wsID, _, _ string) error {
	return f.rec(caller, wsID)
}

type adminRoute struct {
	name    string
	method  string
	body    string
	params  []string
	handler func(h *AdminHandler) echo.HandlerFunc
}

func adminRoutes() []adminRoute {
	return []adminRoute{
		{"CreateDepartment", http.MethodPost, `{"name":"Sales"}`, []string{"id"}, func(h *AdminHandler) echo.HandlerFunc { return h.CreateDepartment }},
		{"ListDepartments", http.MethodGet, "", []string{"id"}, func(h *AdminHandler) echo.HandlerFunc { return h.ListDepartments }},
		{"UpdateDepartment", http.MethodPut, `{"name":"Ops"}`, []string{"id", "deptId"}, func(h *AdminHandler) echo.HandlerFunc { return h.UpdateDepartment }},
		{"DeleteDepartment", http.MethodDelete, "", []string{"id", "deptId"}, func(h *AdminHandler) echo.HandlerFunc { return h.DeleteDepartment }},
		{"MoveDepartment", http.MethodPut, `{"new_parent_id":""}`, []string{"id", "deptId"}, func(h *AdminHandler) echo.HandlerFunc { return h.MoveDepartment }},
		{"UpdateMemberDepartment", http.MethodPut, `{"department_id":"d"}`, []string{"id", "nodeId"}, func(h *AdminHandler) echo.HandlerFunc { return h.UpdateMemberDepartment }},
	}
}

func newAdminCtx(t *testing.T, r adminRoute, withClaims bool) echo.Context {
	t.Helper()
	c, _ := newCtx(t, route{method: r.method, body: r.body, params: r.params}, withClaims)
	return c
}

func TestAdminHandler_PassesCallerAndWorkspace(t *testing.T) {
	for _, r := range adminRoutes() {
		t.Run(r.name, func(t *testing.T) {
			f := &fakeDeptService{}
			require.NoError(t, r.handler(NewAdminHandler(f))(newAdminCtx(t, r, true)))
			assert.Equal(t, callerNode, f.caller)
			assert.Equal(t, "ws-1", f.wsID)
		})
	}
}

func TestAdminHandler_AccessDeniedIs403(t *testing.T) {
	for _, r := range adminRoutes() {
		t.Run(r.name, func(t *testing.T) {
			f := &fakeDeptService{err: domain.ErrAccessDenied}
			err := r.handler(NewAdminHandler(f))(newAdminCtx(t, r, true))
			var he *echo.HTTPError
			require.True(t, errors.As(err, &he), "err = %v", err)
			assert.Equal(t, http.StatusForbidden, he.Code)
		})
	}
}

func TestAdminHandler_MissingClaimsIs401(t *testing.T) {
	for _, r := range adminRoutes() {
		t.Run(r.name, func(t *testing.T) {
			f := &fakeDeptService{}
			err := r.handler(NewAdminHandler(f))(newAdminCtx(t, r, false))
			var he *echo.HTTPError
			require.True(t, errors.As(err, &he), "err = %v", err)
			assert.Equal(t, http.StatusUnauthorized, he.Code)
			assert.Zero(t, f.calls)
		})
	}
}
