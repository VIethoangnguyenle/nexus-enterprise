package rest

import (
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	workspacepb "ngac-platform/proto/workspace"
	"ngac-platform/services/auth/internal/store"
)

const resolvePath = "/api/workspaces/w1/contacts/resolve"

func TestContactsResolve_NameAndAvatarOnly(t *testing.T) {
	a := contactsApp(t)
	a.st.resolved = []store.ResolvedPerson{{UserID: "gone", NGACNodeID: "node-gone", DisplayName: "Đã Rời", AvatarURL: "/avatars/gone"}}

	rec := a.do(http.MethodPost, resolvePath, a.token("member"), `{"user_ids":["gone"],"node_ids":["node-x"]}`)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	people := decode(t, rec)["people"].([]any)
	require.Len(t, people, 1)
	person := people[0].(map[string]any)
	assert.Equal(t, map[string]any{"user_id": "gone", "ngac_node_id": "node-gone", "display_name": "Đã Rời", "avatar_url": "/avatars/gone"}, person,
		"exactly a name and a picture: no e-mail, title, department or handle")
}

func TestContactsResolve_NothingFoundIsAnEmptyListNotNull(t *testing.T) {
	a := contactsApp(t)
	rec := a.do(http.MethodPost, resolvePath, a.token("member"), `{"user_ids":["ghost"]}`)
	require.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, `{"people":[]}`, rec.Body.String())
}

func TestContactsResolve_OnlyAnActiveMemberMayAsk(t *testing.T) {
	a := contactsApp(t)
	a.st.add("outsider", "o@example.test")
	a.st.add("gone", "g@example.test")
	a.st.members["gone"] = []store.WorkspaceSummary{{ID: "w1", Name: "Khối", Role: "member"}}
	a.st.status["gone"] = "removed"
	a.st.resolved = []store.ResolvedPerson{{UserID: "p", DisplayName: "Ai Đó"}}
	body := `{"user_ids":["p"]}`

	rec := a.do(http.MethodPost, resolvePath, a.token("member"), body)
	require.Equal(t, http.StatusOK, rec.Code, "allow: the member of the same fixture")
	require.Equal(t, 1, a.st.resolveCall)

	for who, tok := range map[string]string{"outsider": a.token("outsider"), "removed": a.token("gone")} {
		rec := a.do(http.MethodPost, resolvePath, tok, body)
		assert.Equal(t, http.StatusForbidden, rec.Code, who)
		assert.Equal(t, "access_denied", decode(t, rec)["code"], who)
		assert.NotContains(t, rec.Body.String(), "Ai Đó", who+": the refusal must not carry a name")
	}
	assert.Equal(t, 1, a.st.resolveCall, "the store was not asked for a refused caller")
	assert.Equal(t, http.StatusUnauthorized, a.do(http.MethodPost, resolvePath, "", body).Code)
	assert.Zero(t, a.ws.scopeCalls, "the resolver never asks the workspace service")
}

func TestContactsResolve_RefusesWhatIsNotAnAsk(t *testing.T) {
	a := contactsApp(t)
	var many []string
	for i := 0; i < 201; i++ {
		many = append(many, `"u`+strings.Repeat("a", i%7)+string(rune('a'+i%26))+`"`)
	}
	for name, body := range map[string]string{
		"not json":       `nope`,
		"201 ids":        `{"user_ids":[` + strings.Join(many, ",") + `]}`,
		"split over two": `{"user_ids":[` + strings.Join(many[:100], ",") + `],"node_ids":[` + strings.Join(many[100:], ",") + `]}`,
	} {
		rec := a.do(http.MethodPost, resolvePath, a.token("member"), body)
		assert.Equal(t, http.StatusBadRequest, rec.Code, name)
	}
	assert.Zero(t, a.st.resolveCall)

	two := strings.Join(many[:200], ",")
	assert.Equal(t, http.StatusOK, a.do(http.MethodPost, resolvePath, a.token("member"), `{"user_ids":[`+two+`]}`).Code, "allow: exactly 200")
}

// --- GET contacts: the scope on the wire ---

func TestContacts_CarriesDepartmentRolesAndVisibility(t *testing.T) {
	a := contactsApp(t)
	a.ws.scope = &workspacepb.DirectoryScope{
		Ready: true, Limited: true, VisibleDepartmentIds: []string{"d1"}, OwnersVisible: true,
		Roles: []*workspacepb.DirectoryRole{
			{Id: "o", Kind: "owners", HolderNodeIds: []string{"node-1"}},
			{Id: "r", Name: "Kế toán", Kind: "custom", HolderNodeIds: []string{"node-1"}},
		},
	}
	a.st.contacts = []store.User{{ID: "c1", NGACNodeID: "node-1", DisplayName: "Lan", DepartmentID: "d1", Department: "Kinh doanh"}, {ID: "c2", NGACNodeID: "node-2"}}

	rec := a.do(http.MethodGet, "/api/workspaces/w1/contacts?department_id=d1", a.token("member"), "")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	body := decode(t, rec)
	assert.Equal(t, "limited", body["visibility"])
	list := body["contacts"].([]any)
	first, second := list[0].(map[string]any), list[1].(map[string]any)
	assert.Equal(t, "d1", first["department_id"])
	assert.Equal(t, []any{
		map[string]any{"id": "o", "kind": "owners"},
		map[string]any{"id": "r", "name": "Kế toán", "kind": "custom"},
	}, first["roles"], "the owners group has no name on the wire")
	assert.Equal(t, []any{}, second["roles"], "no roles is an empty list")
	assert.Equal(t, "", second["department_id"])
	assert.Equal(t, "d1", a.st.lastFilter.DepartmentID)
	assert.False(t, a.st.lastFilter.Scope.All)
}

func TestContacts_VisibilityIsAllWithoutRules(t *testing.T) {
	a := contactsApp(t)
	rec := a.do(http.MethodGet, "/api/workspaces/w1/contacts", a.token("member"), "")
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "all", decode(t, rec)["visibility"])
	assert.True(t, a.st.lastFilter.Scope.All)
}

func TestContacts_WorkspaceNotAnsweringIsA503AndNoDirectory(t *testing.T) {
	a := contactsApp(t)
	a.ws.scopeErr = status.Error(codes.Unavailable, "down")
	rec := a.do(http.MethodGet, "/api/workspaces/w1/contacts", a.token("member"), "")
	assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
	assert.Nil(t, a.st.lastFilter, "the directory was not read")
	assert.NotContains(t, rec.Body.String(), "Đồng nghiệp")

	// the same fixture once the workspace answers
	a.ws.scopeErr = nil
	assert.Equal(t, http.StatusOK, a.do(http.MethodGet, "/api/workspaces/w1/contacts", a.token("member"), "").Code)
}

func TestContacts_WorkspaceRefusingTheCallerIs403(t *testing.T) {
	a := contactsApp(t)
	a.ws.scopeErr = status.Error(codes.PermissionDenied, "not a member")
	rec := a.do(http.MethodGet, "/api/workspaces/w1/contacts", a.token("member"), "")
	assert.Equal(t, http.StatusForbidden, rec.Code)
	assert.Nil(t, a.st.lastFilter)
}

func TestContactsResolve_ABodyOverTheLimitIs413(t *testing.T) {
	a := contactsApp(t)
	big := `{"user_ids":["` + strings.Repeat("a", 70<<10) + `"]}`
	rec := a.do(http.MethodPost, resolvePath, a.token("member"), big)
	assert.Equal(t, http.StatusRequestEntityTooLarge, rec.Code)
	assert.Zero(t, a.st.resolveCall)
	assert.Equal(t, http.StatusBadRequest, a.do(http.MethodPost, resolvePath, a.token("member"), `nope`).Code, "malformed stays 400")
}
