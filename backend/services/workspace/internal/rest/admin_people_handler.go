package rest

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"

	"ngac-platform/ngac"
	"ngac-platform/pkg/httputil"
	"ngac-platform/services/workspace/internal/domain"
)

// PeopleService is what the Users and Roles screens call. The caller is the
// NGAC user node ID from verified claims; the workspace comes from the route.
type PeopleService interface {
	ListPermissionAreas(ctx context.Context, caller, wsID string) ([]domain.PermissionArea, error)
	ListRoleSummaries(ctx context.Context, caller, wsID string) ([]*domain.RoleSummary, error)
	CreateRole(ctx context.Context, caller, wsID, name string) (*domain.Role, error)
	GetRoleDetail(ctx context.Context, caller, wsID, roleID string) (*domain.RoleDetail, error)
	DeleteRole(ctx context.Context, caller, wsID, roleID string) error
	SetRolePermissions(ctx context.Context, caller, wsID, roleID string, area ngac.Area, ops []string) (*domain.AreaGrant, error)
	AssignMemberRole(ctx context.Context, caller, wsID, targetNodeID, roleID string) error
	UnassignMemberRole(ctx context.Context, caller, wsID, targetNodeID, roleID string) error
	ListMemberDirectory(ctx context.Context, caller, wsID string) ([]*domain.MemberView, error)
	InviteByEmail(ctx context.Context, caller, wsID string, in domain.InviteInput) error
	ListInvitations(ctx context.Context, caller, wsID string) ([]*domain.InvitationView, error)
	RevokeInvitation(ctx context.Context, caller, wsID, invitationID string) error
	// The invitee's side: userID and node come from verified claims.
	ListMyInvitations(ctx context.Context, userID string) ([]*domain.InvitationView, error)
	AcceptInvitation(ctx context.Context, userID, nodeID, invitationID string) (*domain.AcceptResult, error)
	DeclineInvitation(ctx context.Context, userID, invitationID string) error
}

func (h *AdminHandler) registerPeopleRoutes(api *echo.Group) {
	api.GET("/workspaces/:id/permission-areas", h.ListPermissionAreas)
	api.GET("/workspaces/:id/roles", h.ListRoles)
	api.POST("/workspaces/:id/roles", h.CreateRole)
	api.GET("/workspaces/:id/roles/:roleId", h.GetRole)
	api.DELETE("/workspaces/:id/roles/:roleId", h.DeleteRole)
	api.PUT("/workspaces/:id/roles/:roleId/permissions/:area", h.SetRolePermissions)
	api.GET("/workspaces/:id/admin/members", h.ListMemberDirectory)
	api.POST("/workspaces/:id/members", h.InviteByEmail)
	api.GET("/workspaces/:id/invitations", h.ListInvitations)
	api.DELETE("/workspaces/:id/invitations/:invitationId", h.RevokeInvitation)
	api.GET("/invitations", h.ListMyInvitations)
	api.POST("/invitations/:invitationId/accept", h.AcceptInvitation)
	api.POST("/invitations/:invitationId/decline", h.DeclineInvitation)
	api.PUT("/workspaces/:id/members/:nodeId/roles/:roleId", h.AssignMemberRole)
	api.DELETE("/workspaces/:id/members/:nodeId/roles/:roleId", h.UnassignMemberRole)
}

// --- wire shapes: names beside ids, never an id where a name belongs ---

type areaJSON struct {
	Area       string   `json:"area"`
	Operations []string `json:"operations"`
}

type roleJSON struct {
	ID          string `json:"id"`
	Name        string `json:"name,omitempty"`
	NGACNodeID  string `json:"ngac_node_id"`
	Kind        string `json:"kind"`
	MemberCount int    `json:"member_count"`
}

type personJSON struct {
	NGACNodeID  string `json:"ngac_node_id"`
	UserID      string `json:"user_id"`
	DisplayName string `json:"display_name"`
	AvatarURL   string `json:"avatar_url"`
}

type memberJSON struct {
	NGACNodeID  string      `json:"ngac_node_id"`
	UserID      string      `json:"user_id"`
	DisplayName string      `json:"display_name"`
	Email       string      `json:"email"`
	AvatarURL   string      `json:"avatar_url"`
	Title       string      `json:"title"`
	Status      string      `json:"status"`
	IsOwner     bool        `json:"is_owner"`
	Department  *namedJSON  `json:"department"`
	Roles       []namedJSON `json:"roles"`
}

type namedJSON struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

func toRoleJSON(r *domain.RoleSummary) roleJSON {
	return roleJSON{ID: r.ID, Name: r.Name, NGACNodeID: r.ID, Kind: string(r.Kind), MemberCount: r.MemberCount}
}

func toMemberJSON(m *domain.MemberView) memberJSON {
	out := memberJSON{
		NGACNodeID: m.NodeID, UserID: m.UserID, DisplayName: m.DisplayName, Email: m.Email,
		AvatarURL: m.AvatarURL, Title: m.Title, Status: m.Status, IsOwner: m.Owner,
		Roles: make([]namedJSON, 0, len(m.Roles)),
	}
	if m.Department != nil {
		out.Department = &namedJSON{ID: m.Department.ID, Name: m.Department.Name}
	}
	for _, r := range m.Roles {
		out.Roles = append(out.Roles, namedJSON{ID: r.ID, Name: r.Name})
	}
	return out
}

// ListPermissionAreas handles GET /api/workspaces/:id/permission-areas: the
// areas a role can be granted on and the operations valid on each. The answer
// is the server's; clients do not carry their own table.
func (h *AdminHandler) ListPermissionAreas(c echo.Context) error {
	claims, err := httputil.RequireClaims(c)
	if err != nil {
		return err
	}
	areas, err := h.domain.ListPermissionAreas(c.Request().Context(), claims.NGACNodeID, c.Param("id"))
	if err != nil {
		return httputil.MapDomainError(err)
	}
	out := make([]areaJSON, 0, len(areas))
	for _, a := range areas {
		out = append(out, areaJSON{Area: string(a.Area), Operations: a.Operations})
	}
	return c.JSON(http.StatusOK, map[string]any{"areas": out})
}

// ListRoles handles GET /api/workspaces/:id/roles. `roles` are the
// administrator's own (what pickers elsewhere list); the two built-in roles
// come apart in `system_roles`.
func (h *AdminHandler) ListRoles(c echo.Context) error {
	claims, err := httputil.RequireClaims(c)
	if err != nil {
		return err
	}
	list, err := h.domain.ListRoleSummaries(c.Request().Context(), claims.NGACNodeID, c.Param("id"))
	if err != nil {
		return httputil.MapDomainError(err)
	}
	custom := make([]roleJSON, 0, len(list))
	system := make([]roleJSON, 0, 2)
	for _, r := range list {
		if r.Kind == domain.RoleCustom {
			custom = append(custom, toRoleJSON(r))
		} else {
			system = append(system, toRoleJSON(r))
		}
	}
	return c.JSON(http.StatusOK, map[string]any{"roles": custom, "system_roles": system})
}

// CreateRole handles POST /api/workspaces/:id/roles.
func (h *AdminHandler) CreateRole(c echo.Context) error {
	claims, err := httputil.RequireClaims(c)
	if err != nil {
		return err
	}
	var body struct {
		Name string `json:"name"`
	}
	if err := c.Bind(&body); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}
	r, err := h.domain.CreateRole(c.Request().Context(), claims.NGACNodeID, c.Param("id"), body.Name)
	if err != nil {
		return httputil.MapDomainError(err)
	}
	return c.JSON(http.StatusCreated, toRoleJSON(&domain.RoleSummary{ID: r.ID, Name: r.Name, Kind: domain.RoleCustom}))
}

// GetRole handles GET /api/workspaces/:id/roles/:roleId.
func (h *AdminHandler) GetRole(c echo.Context) error {
	claims, err := httputil.RequireClaims(c)
	if err != nil {
		return err
	}
	d, err := h.domain.GetRoleDetail(c.Request().Context(), claims.NGACNodeID, c.Param("id"), c.Param("roleId"))
	if err != nil {
		return httputil.MapDomainError(err)
	}
	members := make([]personJSON, 0, len(d.Members))
	for _, m := range d.Members {
		members = append(members, personJSON{NGACNodeID: m.NodeID, UserID: m.UserID, DisplayName: m.DisplayName, AvatarURL: m.AvatarURL})
	}
	grants := make([]areaJSON, 0, len(d.Grants))
	for _, g := range d.Grants {
		grants = append(grants, areaJSON{Area: string(g.Area), Operations: g.Operations})
	}
	return c.JSON(http.StatusOK, map[string]any{
		"role":        toRoleJSON(&d.RoleSummary),
		"members":     members,
		"permissions": grants,
	})
}

// DeleteRole handles DELETE /api/workspaces/:id/roles/:roleId.
func (h *AdminHandler) DeleteRole(c echo.Context) error {
	claims, err := httputil.RequireClaims(c)
	if err != nil {
		return err
	}
	if err := h.domain.DeleteRole(c.Request().Context(), claims.NGACNodeID, c.Param("id"), c.Param("roleId")); err != nil {
		return httputil.MapDomainError(err)
	}
	return c.NoContent(http.StatusNoContent)
}

// SetRolePermissions handles PUT /api/workspaces/:id/roles/:roleId/permissions/:area.
// The body is the whole set of operations the role holds on that area.
func (h *AdminHandler) SetRolePermissions(c echo.Context) error {
	claims, err := httputil.RequireClaims(c)
	if err != nil {
		return err
	}
	var body struct {
		Operations []string `json:"operations"`
	}
	if err := c.Bind(&body); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}
	g, err := h.domain.SetRolePermissions(c.Request().Context(), claims.NGACNodeID, c.Param("id"), c.Param("roleId"),
		ngac.Area(c.Param("area")), body.Operations)
	if err != nil {
		return httputil.MapDomainError(err)
	}
	ops := g.Operations
	if ops == nil {
		ops = []string{}
	}
	return c.JSON(http.StatusOK, areaJSON{Area: string(g.Area), Operations: ops})
}

// ListMemberDirectory handles GET /api/workspaces/:id/admin/members.
func (h *AdminHandler) ListMemberDirectory(c echo.Context) error {
	claims, err := httputil.RequireClaims(c)
	if err != nil {
		return err
	}
	people, err := h.domain.ListMemberDirectory(c.Request().Context(), claims.NGACNodeID, c.Param("id"))
	if err != nil {
		return httputil.MapDomainError(err)
	}
	out := make([]memberJSON, 0, len(people))
	for _, m := range people {
		out = append(out, toMemberJSON(m))
	}
	return c.JSON(http.StatusOK, map[string]any{"members": out})
}

// mapPeopleError is MapDomainError plus 429 for an inviter over their budget.
func mapPeopleError(err error) error {
	if errors.Is(err, domain.ErrRateLimited) {
		return echo.NewHTTPError(http.StatusTooManyRequests, err.Error())
	}
	return httputil.MapDomainError(err)
}

type invitationJSON struct {
	ID             string `json:"id"`
	Email          string `json:"email,omitempty"`
	WorkspaceName  string `json:"workspace_name,omitempty"`
	InviterName    string `json:"inviter_name"`
	RoleName       string `json:"role_name"`
	DepartmentName string `json:"department_name"`
	CreatedAt      string `json:"created_at"`
	ExpiresAt      string `json:"expires_at"`
}

func toInvitationJSON(v *domain.InvitationView, withEmail bool) invitationJSON {
	out := invitationJSON{
		ID: v.ID, WorkspaceName: v.WorkspaceName, InviterName: v.InviterName, RoleName: v.RoleName,
		DepartmentName: v.DepartmentName, CreatedAt: v.CreatedAt.UTC().Format(time.RFC3339), ExpiresAt: v.ExpiresAt.UTC().Format(time.RFC3339),
	}
	if withEmail {
		out.Email = v.Email
	}
	return out
}

// InviteByEmail handles POST /api/workspaces/:id/members. It records a pending
// invitation and answers 202 {"status":"invited"} for any well-formed address:
// the answer says nothing about whether an account or a membership exists.
func (h *AdminHandler) InviteByEmail(c echo.Context) error {
	claims, err := httputil.RequireClaims(c)
	if err != nil {
		return err
	}
	var body struct {
		Email        string `json:"email"`
		RoleID       string `json:"role_id"`
		DepartmentID string `json:"department_id"`
	}
	if err := c.Bind(&body); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}
	if err := h.domain.InviteByEmail(c.Request().Context(), claims.NGACNodeID, c.Param("id"), domain.InviteInput{
		Email: body.Email, RoleID: body.RoleID, DepartmentID: body.DepartmentID,
	}); err != nil {
		return mapPeopleError(err)
	}
	return c.JSON(http.StatusAccepted, map[string]string{"status": "invited"})
}

// ListInvitations handles GET /api/workspaces/:id/invitations.
func (h *AdminHandler) ListInvitations(c echo.Context) error {
	claims, err := httputil.RequireClaims(c)
	if err != nil {
		return err
	}
	list, err := h.domain.ListInvitations(c.Request().Context(), claims.NGACNodeID, c.Param("id"))
	if err != nil {
		return mapPeopleError(err)
	}
	out := make([]invitationJSON, 0, len(list))
	for _, v := range list {
		out = append(out, toInvitationJSON(v, true))
	}
	return c.JSON(http.StatusOK, map[string]any{"invitations": out})
}

// RevokeInvitation handles DELETE /api/workspaces/:id/invitations/:invitationId.
func (h *AdminHandler) RevokeInvitation(c echo.Context) error {
	claims, err := httputil.RequireClaims(c)
	if err != nil {
		return err
	}
	if err := h.domain.RevokeInvitation(c.Request().Context(), claims.NGACNodeID, c.Param("id"), c.Param("invitationId")); err != nil {
		return mapPeopleError(err)
	}
	return c.NoContent(http.StatusNoContent)
}

// ListMyInvitations handles GET /api/invitations: the open offers addressed to
// the caller's own account.
func (h *AdminHandler) ListMyInvitations(c echo.Context) error {
	claims, err := httputil.RequireClaims(c)
	if err != nil {
		return err
	}
	list, err := h.domain.ListMyInvitations(c.Request().Context(), claims.UserID)
	if err != nil {
		return mapPeopleError(err)
	}
	out := make([]invitationJSON, 0, len(list))
	for _, v := range list {
		out = append(out, toInvitationJSON(v, false))
	}
	return c.JSON(http.StatusOK, map[string]any{"invitations": out})
}

// AcceptInvitation handles POST /api/invitations/:invitationId/accept.
func (h *AdminHandler) AcceptInvitation(c echo.Context) error {
	claims, err := httputil.RequireClaims(c)
	if err != nil {
		return err
	}
	res, err := h.domain.AcceptInvitation(c.Request().Context(), claims.UserID, claims.NGACNodeID, c.Param("invitationId"))
	if err != nil {
		return mapPeopleError(err)
	}
	return c.JSON(http.StatusOK, map[string]any{
		"workspace_id": res.WorkspaceID, "workspace_name": res.WorkspaceName,
		"role_applied": res.RoleApplied, "department_applied": res.DepartmentApplied,
	})
}

// DeclineInvitation handles POST /api/invitations/:invitationId/decline.
func (h *AdminHandler) DeclineInvitation(c echo.Context) error {
	claims, err := httputil.RequireClaims(c)
	if err != nil {
		return err
	}
	if err := h.domain.DeclineInvitation(c.Request().Context(), claims.UserID, c.Param("invitationId")); err != nil {
		return mapPeopleError(err)
	}
	return c.NoContent(http.StatusNoContent)
}

// AssignMemberRole handles PUT /api/workspaces/:id/members/:nodeId/roles/:roleId.
func (h *AdminHandler) AssignMemberRole(c echo.Context) error {
	claims, err := httputil.RequireClaims(c)
	if err != nil {
		return err
	}
	if err := h.domain.AssignMemberRole(c.Request().Context(), claims.NGACNodeID, c.Param("id"), c.Param("nodeId"), c.Param("roleId")); err != nil {
		return httputil.MapDomainError(err)
	}
	return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
}

// UnassignMemberRole handles DELETE /api/workspaces/:id/members/:nodeId/roles/:roleId.
func (h *AdminHandler) UnassignMemberRole(c echo.Context) error {
	claims, err := httputil.RequireClaims(c)
	if err != nil {
		return err
	}
	if err := h.domain.UnassignMemberRole(c.Request().Context(), claims.NGACNodeID, c.Param("id"), c.Param("nodeId"), c.Param("roleId")); err != nil {
		return httputil.MapDomainError(err)
	}
	return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
}
