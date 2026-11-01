// Package rest provides Echo REST handlers for the auth service.
// Handles client-facing HTTP/JSON for signup, signin, tenant switching, and user queries.
// Delegates all logic to the domain layer.
package rest

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"

	"ngac-platform/pkg/httputil"
	"ngac-platform/services/auth/internal/domain"
)

// UpdateProfile handles PATCH /api/me/profile. It is a partial update: a field
// that is absent (or null) is left as it is, and "" clears it. The person
// edited is always the token's; nothing in the body can name another.
//
// The title belongs to a workspace: ?workspace=<id> makes it that workspace's,
// and is refused (403) unless the caller is an active member there. Without
// it the title written is the account's default, which every workspace where
// none was set shows. The token's workspace is deliberately not used: the
// profile step of sign-up writes the default with whatever token it holds.
func (h *Handler) UpdateProfile(c echo.Context) error {
	claims, err := httputil.RequireClaims(c)
	if err != nil {
		return err
	}

	var body struct {
		DisplayName *string `json:"display_name"`
		Title       *string `json:"title"`
		Location    *string `json:"location"`
		Language    *string `json:"language"`
		// The two fields below are read only to be refused by name. A picture
		// is uploaded to PUT /api/me/avatar, which makes its address itself.
		Department *string `json:"department"`
		AvatarURL  *string `json:"avatar_url"`
	}
	if err := c.Bind(&body); err != nil {
		return badBody()
	}
	// A department is assigned by an administrator, per workspace; letting
	// people write their own would let anyone present themselves as "Ban giám
	// đốc" in the directory. Refused, not ignored, so a client that sends it
	// learns it does not work.
	if body.Department != nil {
		return apiError(http.StatusBadRequest, "department_not_editable",
			"the department is assigned by a workspace administrator")
	}
	// A URL the person typed would be loaded by every colleague's browser.
	if body.AvatarURL != nil {
		return apiError(http.StatusBadRequest, "avatar_not_supported", "upload the picture to PUT /api/me/avatar")
	}

	if err := h.svc.UpdateProfile(c.Request().Context(), claims.UserID, domain.ProfileUpdateInput{
		DisplayName: body.DisplayName,
		Title:       body.Title,
		Location:    body.Location,
		Language:    body.Language,
		WorkspaceID: c.QueryParam("workspace"),
	}); err != nil {
		return fail(c, err)
	}

	return c.JSON(http.StatusOK, map[string]string{"status": "updated"})
}

// workspaceJSON is a workspace as the picker needs it. The identifier is for
// navigation; no screen prints it.
func workspaceJSON(w domain.WorkspaceInfo) map[string]any {
	return map[string]any{"id": w.ID, "name": w.Name, "role": w.Role, "member_count": w.MemberCount, "domain": w.Domain, "type": w.Type}
}

// ListMyWorkspaces handles GET /api/me/workspaces: the workspaces the caller
// actively belongs to, with their own role and the headcount.
func (h *Handler) ListMyWorkspaces(c echo.Context) error {
	claims, err := httputil.RequireClaims(c)
	if err != nil {
		return err
	}
	list, err := h.svc.ListMyWorkspacesAt(c.Request().Context(), claims.UserID, claims.NGACNodeID, claims.TenantID)
	if err != nil {
		return fail(c, err)
	}
	out := make([]map[string]any, len(list))
	for i, w := range list {
		out[i] = workspaceJSON(w)
	}
	return c.JSON(http.StatusOK, map[string]any{"workspaces": out})
}

// CreateMyWorkspace handles POST /api/me/workspaces. Body: name and type
// ("personal" or "organization"); the caller becomes the owner and the
// workspace is provisioned in full. A client that sends no type (one built
// before the choice existed) creates an organization, as it always did.
func (h *Handler) CreateMyWorkspace(c echo.Context) error {
	claims, err := httputil.RequireClaims(c)
	if err != nil {
		return err
	}
	var body struct {
		Name string  `json:"name"`
		Type *string `json:"type"`
	}
	if err := c.Bind(&body); err != nil {
		return badBody()
	}
	wsType := domain.WorkspaceOrganization
	if body.Type != nil {
		wsType = *body.Type
	}
	ws, err := h.svc.CreateMyWorkspace(c.Request().Context(), claims.UserID, claims.NGACNodeID, body.Name, wsType)
	if err != nil {
		return fail(c, err)
	}
	return c.JSON(http.StatusCreated, workspaceJSON(*ws))
}

// ListContacts handles GET /api/workspaces/:id/contacts: one page of the
// workspace directory, as the caller may see it (the workspace decides, auth
// filters). Query: department_id, department (by name), location, search, limit
// (default 50, at most 500) and cursor (the next_cursor of the page before). The
// answer carries the true total, next_cursor when more people follow, and
// visibility: "limited" when the caller sees only part of the workspace.
func (h *Handler) ListContacts(c echo.Context) error {
	wsID := c.Param("id")
	if wsID == "" {
		return apiError(http.StatusBadRequest, "invalid_input", "workspace id required")
	}
	claims, err := httputil.RequireClaims(c)
	if err != nil {
		return err
	}
	q := domain.ContactQuery{
		DepartmentID: c.QueryParam("department_id"),
		Department:   c.QueryParam("department"),
		Location:     c.QueryParam("location"),
		Search:       c.QueryParam("search"),
		Cursor:       c.QueryParam("cursor"),
	}
	if v := c.QueryParam("limit"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 {
			return apiError(http.StatusBadRequest, "invalid_input", "limit must be a positive number")
		}
		q.Limit = n
	}

	page, err := h.svc.ListContacts(c.Request().Context(), claims.UserID, claims.NGACNodeID, wsID, q)
	if err != nil {
		return fail(c, err)
	}

	type contactJSON struct {
		UserID     string `json:"user_id"`
		NGACNodeID string `json:"ngac_node_id"`
		// Username is the login handle. It identifies a person to the chat's
		// own records; it is never a name, and display_name is empty (not the
		// handle) for someone who has not yet said what to call them.
		Username    string `json:"username"`
		DisplayName string `json:"display_name"`
		Email       string `json:"email"`
		Title       string `json:"title"`
		Department  string `json:"department"`
		Location    string `json:"location"`
		AvatarURL   string `json:"avatar_url"`
		// DepartmentID is the department assigned in this workspace ("" for none).
		DepartmentID string `json:"department_id"`
		// Roles are the ones the caller may see this person hold; always a list.
		Roles []roleJSON `json:"roles"`
	}
	contacts, total, next := page.Contacts, page.Total, page.NextCursor
	result := make([]contactJSON, len(contacts))
	for i, c := range contacts {
		result[i] = contactJSON{
			UserID: c.UserID, NGACNodeID: c.NGACNodeID, Username: c.Username,
			DisplayName: c.DisplayName, Email: c.Email,
			Title: c.Title, Department: c.Department,
			Location: c.Location, AvatarURL: c.AvatarURL,
			DepartmentID: c.DepartmentID, Roles: rolesJSON(c.Roles),
		}
	}

	out := map[string]any{"contacts": result, "total": total, "visibility": page.Visibility}
	if next != "" {
		out["next_cursor"] = next
	}
	return c.JSON(http.StatusOK, out)
}

// roleJSON is a role on a person. The owners group has kind "owners" and no name: the
// screen words it.
type roleJSON struct {
	ID   string `json:"id"`
	Name string `json:"name,omitempty"`
	Kind string `json:"kind"`
}

func rolesJSON(in []domain.ContactRole) []roleJSON {
	out := make([]roleJSON, len(in))
	for i, r := range in {
		out[i] = roleJSON{ID: r.ID, Name: r.Name, Kind: r.Kind}
	}
	return out
}

// resolveBodyLimit is the most a resolve request may carry: 200 ids fit many times over.
const resolveBodyLimit = 64 << 10

// ResolveContacts handles POST /api/workspaces/:id/contacts/resolve. Body: user_ids and/or
// node_ids (at most 200 in all). It answers {"people": [{user_id, ngac_node_id,
// display_name, avatar_url}]}: a name and a picture for each person who ever belonged to
// the workspace, whatever their status or directory visibility, and nothing else. Ids it
// does not know are left out. Any member of the workspace may ask.
func (h *Handler) ResolveContacts(c echo.Context) error {
	wsID := c.Param("id")
	if wsID == "" {
		return apiError(http.StatusBadRequest, "invalid_input", "workspace id required")
	}
	claims, err := httputil.RequireClaims(c)
	if err != nil {
		return err
	}
	var body struct {
		UserIDs []string `json:"user_ids"`
		NodeIDs []string `json:"node_ids"`
	}
	c.Request().Body = http.MaxBytesReader(c.Response(), c.Request().Body, resolveBodyLimit)
	if err := c.Bind(&body); err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			return apiError(http.StatusRequestEntityTooLarge, "too_large", "request body too large")
		}
		return badBody()
	}
	people, err := h.svc.ResolvePeople(c.Request().Context(), claims.UserID, claims.NGACNodeID, wsID, body.UserIDs, body.NodeIDs)
	if err != nil {
		return fail(c, err)
	}
	type personJSON struct {
		UserID      string `json:"user_id"`
		NGACNodeID  string `json:"ngac_node_id"`
		DisplayName string `json:"display_name"`
		AvatarURL   string `json:"avatar_url"`
	}
	out := make([]personJSON, len(people))
	for i, p := range people {
		out[i] = personJSON{UserID: p.UserID, NGACNodeID: p.NGACNodeID, DisplayName: p.DisplayName, AvatarURL: p.AvatarURL}
	}
	return c.JSON(http.StatusOK, map[string]any{"people": out})
}
