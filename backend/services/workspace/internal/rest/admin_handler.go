package rest

import (
	"context"
	"net/http"

	"github.com/labstack/echo/v4"

	"ngac-platform/pkg/httputil"
	"ngac-platform/services/workspace/internal/domain"
)

// DepartmentService defines operations the admin handler needs. Every call
// takes the caller's NGAC user node ID, taken from verified JWT claims, and the
// workspace from the route; authorization happens in the domain.
type DepartmentService interface {
	CreateDepartment(ctx context.Context, callerNodeID string, in domain.CreateDepartmentInput) (*domain.DepartmentResult, error)
	ListDepartments(ctx context.Context, callerNodeID, wsID string) ([]*domain.DepartmentResult, error)
	UpdateDepartment(ctx context.Context, callerNodeID, wsID, deptID, newName string) (*domain.DepartmentResult, error)
	DeleteDepartment(ctx context.Context, callerNodeID, wsID, deptID string) error
	MoveDepartment(ctx context.Context, callerNodeID string, in domain.MoveDepartmentInput) (*domain.DepartmentResult, error)
	UpdateMemberDepartment(ctx context.Context, callerNodeID, wsID, userNGACNodeID, deptID string) error
}

// AdminService is everything the admin screens call: departments, roles,
// permissions and people. Every call takes the caller's NGAC user node ID from
// verified JWT claims; authorization happens in the domain.
type AdminService interface {
	DepartmentService
	PeopleService
}

// AdminHandler serves admin organization endpoints.
type AdminHandler struct {
	domain AdminService
}

// NewAdminHandler creates an admin REST handler.
func NewAdminHandler(svc AdminService) *AdminHandler {
	return &AdminHandler{domain: svc}
}

// RegisterAdminRoutes mounts admin endpoints.
func (h *AdminHandler) RegisterAdminRoutes(api *echo.Group) {
	api.POST("/workspaces/:id/departments", h.CreateDepartment)
	api.GET("/workspaces/:id/departments", h.ListDepartments)
	api.PUT("/workspaces/:id/departments/:deptId", h.UpdateDepartment)
	api.DELETE("/workspaces/:id/departments/:deptId", h.DeleteDepartment)
	api.PUT("/workspaces/:id/departments/:deptId/move", h.MoveDepartment)
	api.PUT("/workspaces/:id/members/:nodeId/department", h.UpdateMemberDepartment)
	h.registerPeopleRoutes(api)
}

// CreateDepartment handles POST /api/workspaces/:id/departments.
func (h *AdminHandler) CreateDepartment(c echo.Context) error {
	claims, err := httputil.RequireClaims(c)
	if err != nil {
		return err
	}
	var body struct {
		Name     string `json:"name"`
		ParentID string `json:"parent_id"`
	}
	if err := c.Bind(&body); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}

	result, err := h.domain.CreateDepartment(c.Request().Context(), claims.NGACNodeID, domain.CreateDepartmentInput{
		WorkspaceID: c.Param("id"),
		Name:        body.Name,
		ParentID:    body.ParentID,
	})
	if err != nil {
		return httputil.MapDomainError(err)
	}

	return c.JSON(http.StatusCreated, map[string]any{
		"id":        result.ID,
		"name":      result.Name,
		"parent_id": result.ParentID,
	})
}

// ListDepartments handles GET /api/workspaces/:id/departments.
func (h *AdminHandler) ListDepartments(c echo.Context) error {
	claims, err := httputil.RequireClaims(c)
	if err != nil {
		return err
	}
	results, err := h.domain.ListDepartments(c.Request().Context(), claims.NGACNodeID, c.Param("id"))
	if err != nil {
		return httputil.MapDomainError(err)
	}

	type deptJSON struct {
		ID          string `json:"id"`
		Name        string `json:"name"`
		ParentID    string `json:"parent_id"`
		MemberCount int    `json:"member_count"`
	}

	depts := make([]deptJSON, 0, len(results))
	for _, r := range results {
		depts = append(depts, deptJSON{
			ID:          r.ID,
			Name:        r.Name,
			ParentID:    r.ParentID,
			MemberCount: r.MemberCount,
		})
	}

	return c.JSON(http.StatusOK, map[string]any{"departments": depts})
}

// UpdateDepartment handles PUT /api/workspaces/:id/departments/:deptId.
func (h *AdminHandler) UpdateDepartment(c echo.Context) error {
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

	result, err := h.domain.UpdateDepartment(c.Request().Context(), claims.NGACNodeID, c.Param("id"), c.Param("deptId"), body.Name)
	if err != nil {
		return httputil.MapDomainError(err)
	}

	return c.JSON(http.StatusOK, map[string]any{
		"id":        result.ID,
		"name":      result.Name,
		"parent_id": result.ParentID,
	})
}

// DeleteDepartment handles DELETE /api/workspaces/:id/departments/:deptId.
func (h *AdminHandler) DeleteDepartment(c echo.Context) error {
	claims, err := httputil.RequireClaims(c)
	if err != nil {
		return err
	}
	if err := h.domain.DeleteDepartment(c.Request().Context(), claims.NGACNodeID, c.Param("id"), c.Param("deptId")); err != nil {
		return httputil.MapDomainError(err)
	}
	return c.NoContent(http.StatusNoContent)
}

// MoveDepartment handles PUT /api/workspaces/:id/departments/:deptId/move.
func (h *AdminHandler) MoveDepartment(c echo.Context) error {
	claims, err := httputil.RequireClaims(c)
	if err != nil {
		return err
	}
	var body struct {
		NewParentID string `json:"new_parent_id"`
	}
	if err := c.Bind(&body); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}

	result, err := h.domain.MoveDepartment(c.Request().Context(), claims.NGACNodeID, domain.MoveDepartmentInput{
		WorkspaceID: c.Param("id"),
		DeptID:      c.Param("deptId"),
		NewParentID: body.NewParentID,
	})
	if err != nil {
		return httputil.MapDomainError(err)
	}

	return c.JSON(http.StatusOK, map[string]any{
		"id":        result.ID,
		"name":      result.Name,
		"parent_id": result.ParentID,
	})
}

// UpdateMemberDepartment handles PUT /api/workspaces/:id/members/:nodeId/department.
func (h *AdminHandler) UpdateMemberDepartment(c echo.Context) error {
	claims, err := httputil.RequireClaims(c)
	if err != nil {
		return err
	}
	var body struct {
		DepartmentID string `json:"department_id"`
	}
	if err := c.Bind(&body); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}

	if err := h.domain.UpdateMemberDepartment(
		c.Request().Context(),
		claims.NGACNodeID,
		c.Param("id"),
		c.Param("nodeId"),
		body.DepartmentID,
	); err != nil {
		return httputil.MapDomainError(err)
	}

	return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
}
