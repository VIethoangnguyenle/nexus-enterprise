package rest

import (
	"context"
	"net/http"

	"github.com/labstack/echo/v4"

	"ngac-platform/pkg/httputil"
	"ngac-platform/services/workspace/internal/domain"
)

// DirectoryScopeService is what Quản trị › Danh bạ calls: the rules that decide who sees whom
// in Danh bạ. The caller is the NGAC user node from verified claims; the workspace and the
// subject come from the route; the domain authorizes (manage on the Mgmt OA).
type DirectoryScopeService interface {
	DirectoryRules(ctx context.Context, caller, wsID string) (*domain.DirectoryRules, error)
	HideDepartment(ctx context.Context, caller, wsID, deptID string) error
	UnhideDepartment(ctx context.Context, caller, wsID, deptID string) error
	LimitRole(ctx context.Context, caller, wsID, roleID string) error
	UnlimitRole(ctx context.Context, caller, wsID, roleID string) error
	LimitDepartment(ctx context.Context, caller, wsID, deptID string) error
	UnlimitDepartment(ctx context.Context, caller, wsID, deptID string) error
}

func (h *AdminHandler) registerDirectoryScopeRoutes(api *echo.Group) {
	const base = "/workspaces/:id/admin/directory-scope"
	api.GET(base, h.GetDirectoryRules)
	api.PUT(base+"/hidden/:deptId", h.ruleWrite(func(ctx context.Context, caller, ws, id string) error {
		return h.domain.HideDepartment(ctx, caller, ws, id)
	}, "deptId"))
	api.DELETE(base+"/hidden/:deptId", h.ruleWrite(func(ctx context.Context, caller, ws, id string) error {
		return h.domain.UnhideDepartment(ctx, caller, ws, id)
	}, "deptId"))
	api.PUT(base+"/limited/roles/:roleId", h.ruleWrite(func(ctx context.Context, caller, ws, id string) error {
		return h.domain.LimitRole(ctx, caller, ws, id)
	}, "roleId"))
	api.DELETE(base+"/limited/roles/:roleId", h.ruleWrite(func(ctx context.Context, caller, ws, id string) error {
		return h.domain.UnlimitRole(ctx, caller, ws, id)
	}, "roleId"))
	api.PUT(base+"/limited/departments/:deptId", h.ruleWrite(func(ctx context.Context, caller, ws, id string) error {
		return h.domain.LimitDepartment(ctx, caller, ws, id)
	}, "deptId"))
	api.DELETE(base+"/limited/departments/:deptId", h.ruleWrite(func(ctx context.Context, caller, ws, id string) error {
		return h.domain.UnlimitDepartment(ctx, caller, ws, id)
	}, "deptId"))
}

// ruleWrite adapts one rule change to a handler: the caller from claims, the workspace and
// the subject from the route.
func (h *AdminHandler) ruleWrite(do func(ctx context.Context, caller, ws, id string) error, param string) echo.HandlerFunc {
	return func(c echo.Context) error {
		claims, err := httputil.RequireClaims(c)
		if err != nil {
			return err
		}
		if err := do(c.Request().Context(), claims.NGACNodeID, c.Param("id"), c.Param(param)); err != nil {
			return httputil.MapDomainError(err)
		}
		return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
	}
}

type ruleSubjectJSON struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	ParentID    string `json:"parent_id,omitempty"`
	MemberCount int    `json:"member_count"`
}

type limitedJSON struct {
	Kind string `json:"kind"` // "role" or "department"
	ruleSubjectJSON
}

// GetDirectoryRules handles GET /api/workspaces/:id/admin/directory-scope: the hidden
// departments and the limited roles and departments, by name with how many people each
// concerns. "ready": false (still 200) says the workspace cannot carry rules yet, so the
// screen is not offered; an outage is a 503 instead.
func (h *AdminHandler) GetDirectoryRules(c echo.Context) error {
	claims, err := httputil.RequireClaims(c)
	if err != nil {
		return err
	}
	rules, err := h.domain.DirectoryRules(c.Request().Context(), claims.NGACNodeID, c.Param("id"))
	if err != nil {
		return httputil.MapDomainError(err)
	}
	hidden := make([]ruleSubjectJSON, 0, len(rules.HiddenDepartments))
	for _, d := range rules.HiddenDepartments {
		hidden = append(hidden, ruleSubjectJSON(d))
	}
	limited := make([]limitedJSON, 0, len(rules.LimitedRoles)+len(rules.LimitedDepartments))
	for _, r := range rules.LimitedRoles {
		limited = append(limited, limitedJSON{Kind: "role", ruleSubjectJSON: ruleSubjectJSON(r)})
	}
	for _, d := range rules.LimitedDepartments {
		limited = append(limited, limitedJSON{Kind: "department", ruleSubjectJSON: ruleSubjectJSON(d)})
	}
	return c.JSON(http.StatusOK, map[string]any{
		"ready":              rules.Ready,
		"hidden_departments": hidden,
		"limited":            limited,
	})
}
