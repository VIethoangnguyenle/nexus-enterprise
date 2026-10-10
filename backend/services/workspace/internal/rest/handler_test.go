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

	"ngac-platform/ngac"
	"ngac-platform/pkg/grpcauth"
	"ngac-platform/pkg/httputil"
	pb "ngac-platform/proto/workspace"
	"ngac-platform/services/workspace/internal/domain"
)

const callerNode = "u-caller"

// fakeWorkspaceService records what each handler passes down and returns err.
type fakeWorkspaceService struct {
	err       error
	requester string // caller on the context of the last call
	calls     int
}

func (f *fakeWorkspaceService) seen(ctx context.Context) error {
	f.calls++
	f.requester = grpcauth.CallerFrom(ctx).NGACNodeID
	return f.err
}

func (f *fakeWorkspaceService) CreateWorkspace(ctx context.Context, req *pb.CreateWorkspaceRequest) (*pb.Workspace, error) {
	return &pb.Workspace{}, f.seen(ctx)
}
func (f *fakeWorkspaceService) ListWorkspaces(ctx context.Context, req *pb.ListWorkspacesRequest) (*pb.WorkspaceList, error) {
	return &pb.WorkspaceList{}, f.seen(ctx)
}
func (f *fakeWorkspaceService) GetWorkspace(ctx context.Context, _ *pb.GetWorkspaceRequest) (*pb.Workspace, error) {
	return &pb.Workspace{}, f.seen(ctx)
}
func (f *fakeWorkspaceService) RemoveMember(ctx context.Context, req *pb.RemoveMemberRequest) (*pb.Empty, error) {
	return &pb.Empty{}, f.seen(ctx)
}
func (f *fakeWorkspaceService) ListMembers(ctx context.Context, _ *pb.ListMembersRequest) (*pb.MemberList, error) {
	return &pb.MemberList{}, f.seen(ctx)
}
func (f *fakeWorkspaceService) CreateFolder(ctx context.Context, req *pb.CreateFolderRequest) (*pb.Folder, error) {
	return &pb.Folder{}, f.seen(ctx)
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
		{"RemoveMember", http.MethodDelete, "", []string{"id", "nodeId"}, func(h *Handler) echo.HandlerFunc { return h.RemoveMember }},
		{"ListMembers", http.MethodGet, "", []string{"id"}, func(h *Handler) echo.HandlerFunc { return h.ListMembers }},
		{"CreateFolder", http.MethodPost, `{"name":"Legal",` + spoof + `}`, []string{"id"}, func(h *Handler) echo.HandlerFunc { return h.CreateFolder }},
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
	byName := map[string]string{"id": "ws-1", "nodeId": "u-target", "roleId": "ua-role", "deptId": "dept-1", "area": "documents", "invitationId": "inv-1"}
	vals := make([]string, len(r.params))
	for i, name := range r.params {
		vals[i] = byName[name]
	}
	c.SetParamValues(vals...)
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

	lastName, lastRole, lastArea, lastNode, lastEmail, lastDept, lastUser, lastInvitation string
	lastOps                                                                               []string
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

func (f *fakeDeptService) ListPermissionAreas(_ context.Context, caller, wsID string) ([]domain.PermissionArea, error) {
	return []domain.PermissionArea{{Area: ngac.AreaDocuments, Operations: ngac.AreaOps(ngac.AreaDocuments)}}, f.rec(caller, wsID)
}
func (f *fakeDeptService) ListRoleSummaries(_ context.Context, caller, wsID string) ([]*domain.RoleSummary, error) {
	return []*domain.RoleSummary{
		{ID: "ua-owners", Kind: domain.RoleOwners, MemberCount: 2},
		{ID: "ua-members", Kind: domain.RoleMembers, MemberCount: 9},
		{ID: "ua-1", Name: "Kế toán", Kind: domain.RoleCustom, MemberCount: 3},
	}, f.rec(caller, wsID)
}
func (f *fakeDeptService) CreateRole(_ context.Context, caller, wsID, name string) (*domain.Role, error) {
	f.lastName = name
	return &domain.Role{ID: "ua-new", Name: name}, f.rec(caller, wsID)
}
func (f *fakeDeptService) GetRoleDetail(_ context.Context, caller, wsID, roleID string) (*domain.RoleDetail, error) {
	f.lastRole = roleID
	return &domain.RoleDetail{
		RoleSummary: domain.RoleSummary{ID: roleID, Name: "Kế toán", Kind: domain.RoleCustom, MemberCount: 1},
		Members:     []*domain.PersonRef{{NodeID: "n1", UserID: "usr1", DisplayName: "Lê Thị Hoa"}},
		Grants:      []domain.AreaGrant{{Area: ngac.AreaDocuments, Operations: []string{ngac.OpRead}}},
	}, f.rec(caller, wsID)
}
func (f *fakeDeptService) DeleteRole(_ context.Context, caller, wsID, roleID string) error {
	f.lastRole = roleID
	return f.rec(caller, wsID)
}
func (f *fakeDeptService) SetRolePermissions(_ context.Context, caller, wsID, roleID string, area ngac.Area, ops []string) (*domain.AreaGrant, error) {
	f.lastRole, f.lastArea, f.lastOps = roleID, string(area), ops
	return &domain.AreaGrant{Area: area, Operations: ops}, f.rec(caller, wsID)
}
func (f *fakeDeptService) AssignMemberRole(_ context.Context, caller, wsID, nodeID, roleID string) error {
	f.lastNode, f.lastRole = nodeID, roleID
	return f.rec(caller, wsID)
}
func (f *fakeDeptService) UnassignMemberRole(_ context.Context, caller, wsID, nodeID, roleID string) error {
	f.lastNode, f.lastRole = nodeID, roleID
	return f.rec(caller, wsID)
}
func (f *fakeDeptService) ListMemberDirectory(_ context.Context, caller, wsID string) ([]*domain.MemberView, error) {
	return []*domain.MemberView{{
		NodeID: "n1", UserID: "usr1", DisplayName: "Nguyễn Thu Lan", Email: "lan@novapay.vn", Status: "active",
		Department: &domain.DeptRef{ID: "d1", Name: "Đối soát"}, Roles: []domain.RoleRef{{ID: "ua-1", Name: "Kế toán"}},
	}}, f.rec(caller, wsID)
}
func (f *fakeDeptService) InviteByEmail(_ context.Context, caller, wsID string, in domain.InviteInput) error {
	f.lastEmail, f.lastRole = in.Email, in.RoleID
	f.lastDept = in.DepartmentID
	return f.rec(caller, wsID)
}
func (f *fakeDeptService) ListInvitations(_ context.Context, caller, wsID string) ([]*domain.InvitationView, error) {
	return []*domain.InvitationView{{ID: "inv-1", Email: "moi@novapay.vn", InviterName: "Lê Thị Hoa", RoleName: "Kế toán"}}, f.rec(caller, wsID)
}
func (f *fakeDeptService) RevokeInvitation(_ context.Context, caller, wsID, id string) error {
	f.lastInvitation = id
	return f.rec(caller, wsID)
}
func (f *fakeDeptService) ListMyInvitations(_ context.Context, userID string) ([]*domain.InvitationView, error) {
	f.lastUser = userID
	return []*domain.InvitationView{{ID: "inv-1", Email: "moi@novapay.vn", WorkspaceName: "Khối Vận hành", InviterName: "Lê Thị Hoa"}}, f.rec(callerNode, "")
}
func (f *fakeDeptService) AcceptInvitation(_ context.Context, userID, nodeID, id string) (*domain.AcceptResult, error) {
	f.lastUser, f.lastNode, f.lastInvitation = userID, nodeID, id
	return &domain.AcceptResult{WorkspaceID: "ws-1", WorkspaceName: "Khối Vận hành", RoleApplied: true}, f.rec(nodeID, "")
}
func (f *fakeDeptService) DeclineInvitation(_ context.Context, userID, id string) error {
	f.lastUser, f.lastInvitation = userID, id
	return f.rec(callerNode, "")
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
		{"ListPermissionAreas", http.MethodGet, "", []string{"id"}, func(h *AdminHandler) echo.HandlerFunc { return h.ListPermissionAreas }},
		{"ListRoles", http.MethodGet, "", []string{"id"}, func(h *AdminHandler) echo.HandlerFunc { return h.ListRoles }},
		{"CreateRole", http.MethodPost, `{"name":"Kế toán",` + spoof + `}`, []string{"id"}, func(h *AdminHandler) echo.HandlerFunc { return h.CreateRole }},
		{"GetRole", http.MethodGet, "", []string{"id", "roleId"}, func(h *AdminHandler) echo.HandlerFunc { return h.GetRole }},
		{"DeleteRole", http.MethodDelete, "", []string{"id", "roleId"}, func(h *AdminHandler) echo.HandlerFunc { return h.DeleteRole }},
		{"SetRolePermissions", http.MethodPut, `{"operations":["read"],` + spoof + `}`, []string{"id", "roleId", "area"}, func(h *AdminHandler) echo.HandlerFunc { return h.SetRolePermissions }},
		{"ListMemberDirectory", http.MethodGet, "", []string{"id"}, func(h *AdminHandler) echo.HandlerFunc { return h.ListMemberDirectory }},
		{"InviteByEmail", http.MethodPost, `{"email":"moi@novapay.vn","role_id":"r","department_id":"d",` + spoof + `}`, []string{"id"}, func(h *AdminHandler) echo.HandlerFunc { return h.InviteByEmail }},
		{"ListInvitations", http.MethodGet, "", []string{"id"}, func(h *AdminHandler) echo.HandlerFunc { return h.ListInvitations }},
		{"RevokeInvitation", http.MethodDelete, "", []string{"id", "invitationId"}, func(h *AdminHandler) echo.HandlerFunc { return h.RevokeInvitation }},
		{"ListMyInvitations", http.MethodGet, "", nil, func(h *AdminHandler) echo.HandlerFunc { return h.ListMyInvitations }},
		{"AcceptInvitation", http.MethodPost, "", []string{"invitationId"}, func(h *AdminHandler) echo.HandlerFunc { return h.AcceptInvitation }},
		{"DeclineInvitation", http.MethodPost, "", []string{"invitationId"}, func(h *AdminHandler) echo.HandlerFunc { return h.DeclineInvitation }},
		{"AssignMemberRole", http.MethodPut, "", []string{"id", "nodeId", "roleId"}, func(h *AdminHandler) echo.HandlerFunc { return h.AssignMemberRole }},
		{"UnassignMemberRole", http.MethodDelete, "", []string{"id", "nodeId", "roleId"}, func(h *AdminHandler) echo.HandlerFunc { return h.UnassignMemberRole }},
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
			if len(r.params) > 0 && r.params[0] == "id" { // the invitee's routes are not in a workspace
				assert.Equal(t, "ws-1", f.wsID)
			}
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

// ---------------------------------------------------------------------------
// The admin screens' wire shapes and error mapping
// ---------------------------------------------------------------------------

func call(t *testing.T, f *fakeDeptService, name string, route adminRoute) (int, string) {
	t.Helper()
	c, rec := newCtx(t, route2(route), true)
	require.NoError(t, route.handler(NewAdminHandler(f))(c), name)
	return rec.Code, rec.Body.String()
}

func route2(r adminRoute) route {
	return route{method: r.method, body: r.body, params: r.params}
}

func find(name string) adminRoute {
	for _, r := range adminRoutes() {
		if r.name == name {
			return r
		}
	}
	panic("no route " + name)
}

// The server, not the client, says which operations fit which area.
func TestAdminHandler_PermissionAreasComeFromTheServer(t *testing.T) {
	code, body := call(t, &fakeDeptService{}, "areas", find("ListPermissionAreas"))
	assert.Equal(t, http.StatusOK, code)
	assert.JSONEq(t, `{"areas":[{"area":"documents","operations":["read","write","share"]}]}`, body)
}

// Pickers elsewhere list the administrator's roles under `roles`; the two
// built-in ones are apart, and carry no English name.
func TestAdminHandler_RolesSplitCustomFromSystem(t *testing.T) {
	_, body := call(t, &fakeDeptService{}, "roles", find("ListRoles"))
	assert.JSONEq(t, `{
		"roles":[{"id":"ua-1","name":"Kế toán","ngac_node_id":"ua-1","kind":"custom","member_count":3}],
		"system_roles":[
			{"id":"ua-owners","ngac_node_id":"ua-owners","kind":"owners","member_count":2},
			{"id":"ua-members","ngac_node_id":"ua-members","kind":"members","member_count":9}]}`, body)
}

func TestAdminHandler_CreateRoleTakesOnlyTheNameFromTheBody(t *testing.T) {
	f := &fakeDeptService{}
	code, _ := call(t, f, "create", find("CreateRole"))
	assert.Equal(t, http.StatusCreated, code)
	assert.Equal(t, "Kế toán", f.lastName)
	assert.Equal(t, callerNode, f.caller, "the spoofed requester in the body is ignored")
}

func TestAdminHandler_SetRolePermissionsPassesAreaOperationsAndRoleFromTheRouteAndBody(t *testing.T) {
	f := &fakeDeptService{}
	code, body := call(t, f, "set", find("SetRolePermissions"))
	assert.Equal(t, http.StatusOK, code)
	assert.Equal(t, "ua-role", f.lastRole)
	assert.Equal(t, "documents", f.lastArea)
	assert.Equal(t, []string{"read"}, f.lastOps)
	assert.JSONEq(t, `{"area":"documents","operations":["read"]}`, body, "the area is the route's")
	assert.Equal(t, callerNode, f.caller)
}

func TestAdminHandler_SetRolePermissionsAnswersAnEmptyListNotNull(t *testing.T) {
	f := &fakeDeptService{}
	r := find("SetRolePermissions")
	r.body = `{"operations":[]}`
	_, body := call(t, f, "set", r)
	assert.Contains(t, body, `"operations":[]`)
}

func TestAdminHandler_DirectoryRowsCarryNamesAndNoNodeFallbacks(t *testing.T) {
	_, body := call(t, &fakeDeptService{}, "dir", find("ListMemberDirectory"))
	assert.JSONEq(t, `{"members":[{"ngac_node_id":"n1","user_id":"usr1","display_name":"Nguyễn Thu Lan",
		"email":"lan@novapay.vn","avatar_url":"","title":"","status":"active","is_owner":false,
		"department":{"id":"d1","name":"Đối soát"},"roles":[{"id":"ua-1","name":"Kế toán"}]}]}`, body)
}

// Inviting answers 202 and the same body for any address; the caller is the
// token's, and the role and department come from the body only as choices.
func TestAdminHandler_InviteAnswersAcceptedWithNothingAboutTheAddress(t *testing.T) {
	f := &fakeDeptService{}
	code, body := call(t, f, "invite", find("InviteByEmail"))
	assert.Equal(t, http.StatusAccepted, code)
	assert.JSONEq(t, `{"status":"invited"}`, body)
	assert.Equal(t, "moi@novapay.vn", f.lastEmail)
	assert.Equal(t, "r", f.lastRole)
	assert.Equal(t, "d", f.lastDept)
	assert.Equal(t, callerNode, f.caller)
}

func TestAdminHandler_RateLimitIs429(t *testing.T) {
	f := &fakeDeptService{err: domain.ErrRateLimited}
	rt := find("InviteByEmail")
	err := rt.handler(NewAdminHandler(f))(newAdminCtx(t, rt, true))
	var he *echo.HTTPError
	require.True(t, errors.As(err, &he))
	assert.Equal(t, http.StatusTooManyRequests, he.Code)
}

// The invitee is the token's user, never a field of the request.
func TestAdminHandler_InviteeRoutesUseTheVerifiedUser(t *testing.T) {
	f := &fakeDeptService{}
	code, body := call(t, f, "accept", find("AcceptInvitation"))
	assert.Equal(t, http.StatusOK, code)
	assert.Equal(t, "user-1", f.lastUser)
	assert.Equal(t, callerNode, f.lastNode)
	assert.Equal(t, "inv-1", f.lastInvitation)
	assert.JSONEq(t, `{"workspace_id":"ws-1","workspace_name":"Khối Vận hành","role_applied":true,"department_applied":false}`, body)

	f = &fakeDeptService{}
	code, body = call(t, f, "mine", find("ListMyInvitations"))
	assert.Equal(t, http.StatusOK, code)
	assert.Equal(t, "user-1", f.lastUser)
	assert.NotContains(t, body, "moi@novapay.vn", "the invitee's list does not echo addresses")

	f = &fakeDeptService{}
	code, _ = call(t, f, "decline", find("DeclineInvitation"))
	assert.Equal(t, http.StatusNoContent, code)
	assert.Equal(t, "user-1", f.lastUser)
}

func TestAdminHandler_AdminInvitationListShowsAddressAndNames(t *testing.T) {
	_, body := call(t, &fakeDeptService{}, "list", find("ListInvitations"))
	assert.Contains(t, body, `"email":"moi@novapay.vn"`)
	assert.Contains(t, body, `"inviter_name":"Lê Thị Hoa"`)
	assert.Contains(t, body, `"role_name":"Kế toán"`)
}

func TestAdminHandler_InviteeErrorsMap(t *testing.T) {
	for _, tc := range []struct {
		err  error
		code int
	}{{domain.ErrNotFound, 404}, {domain.ErrAlreadyExists, 409}, {domain.ErrInvalidInput, 400}, {domain.ErrAccessDenied, 403}} {
		for _, name := range []string{"AcceptInvitation", "DeclineInvitation"} {
			rt := find(name)
			err := rt.handler(NewAdminHandler(&fakeDeptService{err: tc.err}))(newAdminCtx(t, rt, true))
			var he *echo.HTTPError
			require.True(t, errors.As(err, &he), name)
			assert.Equal(t, tc.code, he.Code, name)
		}
	}
}

func TestAdminHandler_RoleAssignmentUsesRouteIDs(t *testing.T) {
	f := &fakeDeptService{}
	call(t, f, "assign", find("AssignMemberRole"))
	assert.Equal(t, "u-target", f.lastNode)
	assert.Equal(t, "ua-role", f.lastRole)
}

func TestAdminHandler_ErrorsMapToTheirStatuses(t *testing.T) {
	for name, tc := range map[string]struct {
		err  error
		code int
	}{
		"not found":      {domain.ErrNotFound, http.StatusNotFound},
		"denied":         {domain.ErrAccessDenied, http.StatusForbidden},
		"already member": {domain.ErrAlreadyExists, http.StatusConflict},
		"bad input":      {domain.ErrInvalidInput, http.StatusBadRequest},
		"anything else":  {errors.New("boom"), http.StatusInternalServerError},
	} {
		for _, r := range []string{"SetRolePermissions", "InviteByEmail", "AssignMemberRole", "GetRole", "ListMemberDirectory"} {
			f := &fakeDeptService{err: tc.err}
			rt := find(r)
			err := rt.handler(NewAdminHandler(f))(newAdminCtx(t, rt, true))
			var he *echo.HTTPError
			require.True(t, errors.As(err, &he), "%s %s: %v", name, r, err)
			assert.Equal(t, tc.code, he.Code, "%s %s", name, r)
		}
	}
}

func TestAdminHandler_MalformedBodyIs400AndNeverReachesTheService(t *testing.T) {
	for _, r := range []string{"CreateRole", "SetRolePermissions", "InviteByEmail"} {
		f := &fakeDeptService{}
		rt := find(r)
		rt.body = `{not json`
		err := rt.handler(NewAdminHandler(f))(newAdminCtx(t, rt, true))
		var he *echo.HTTPError
		require.True(t, errors.As(err, &he), r)
		assert.Equal(t, http.StatusBadRequest, he.Code, r)
		assert.Zero(t, f.calls, r)
	}
}

// The free-form "grant this UA on that OA" route is gone: it checked manage on
// the route's workspace but not that the UA and OA belonged to it, so with
// re-grants replacing operations it let an owner of one workspace strip
// another's. SetRolePermissions replaces it.
// Adding a person by node ID, with no consent on their side, is gone: people are
// brought in by an invitation they accept.
func TestRoutes_NoDirectAddByNodeID(t *testing.T) {
	e := echo.New()
	NewHandler(&fakeWorkspaceService{}).RegisterRoutes(e, "secret")
	NewAdminHandler(&fakeDeptService{}).RegisterAdminRoutes(e.Group("/api"))
	for _, r := range e.Routes() {
		assert.NotContains(t, r.Path, "/invite", "%s %s", r.Method, r.Path)
	}
	for _, method := range []string{http.MethodPost, http.MethodPut} {
		req := httptest.NewRequest(method, "/api/workspaces/ws-1/invite", strings.NewReader(`{"ngac_node_id":"u-victim"}`))
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		assert.Contains(t, []int{http.StatusNotFound, http.StatusMethodNotAllowed, http.StatusUnauthorized}, rec.Code)
	}
}

func TestRoutes_NoFreeFormPermissionGrant(t *testing.T) {
	e := echo.New()
	NewHandler(&fakeWorkspaceService{}).RegisterRoutes(e, "secret")
	admin := NewAdminHandler(&fakeDeptService{})
	admin.RegisterAdminRoutes(e.Group("/api"))
	for _, r := range e.Routes() {
		assert.NotEqual(t, "/api/workspaces/:id/permissions", r.Path, "%s %s", r.Method, r.Path)
	}
	for _, method := range []string{http.MethodPost, http.MethodPut} {
		req := httptest.NewRequest(method, "/api/workspaces/ws-1/permissions", strings.NewReader(`{"ua_id":"u","oa_id":"o","operations":["read"]}`))
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		assert.Contains(t, []int{http.StatusNotFound, http.StatusMethodNotAllowed, http.StatusUnauthorized}, rec.Code)
		assert.NotEqual(t, http.StatusOK, rec.Code)
	}
}
