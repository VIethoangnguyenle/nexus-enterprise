package rest

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"ngac-platform/services/workspace/internal/domain"
)

// DepartmentService defines operations the admin handler needs.
type DepartmentService interface {
	CreateDepartment(ctx domain.CreateDepartmentInput) (*domain.DepartmentResult, error)
	ListDepartments(wsID string) ([]*domain.DepartmentResult, error)
	UpdateDepartment(deptID, newName string) (*domain.DepartmentResult, error)
	DeleteDepartment(deptID string) error
	MoveDepartment(in domain.MoveDepartmentInput) (*domain.DepartmentResult, error)
	UpdateMemberDepartment(wsID, userNGACNodeID, deptID string) error
}

// AdminHandler serves admin organization endpoints.
type AdminHandler struct {
	domain *domain.Service
}

// NewAdminHandler creates an admin REST handler.
func NewAdminHandler(svc *domain.Service) *AdminHandler {
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
}

// CreateDepartment handles POST /api/workspaces/:id/departments.
func (h *AdminHandler) CreateDepartment(c echo.Context) error {
	var body struct {
		Name     string `json:"name"`
		ParentID string `json:"parent_id"`
	}
	if err := c.Bind(&body); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}

	result, err := h.domain.CreateDepartment(c.Request().Context(), domain.CreateDepartmentInput{
		WorkspaceID: c.Param("id"),
		Name:        body.Name,
		ParentID:    body.ParentID,
	})
	if err != nil {
		return mapDomainError(err)
	}

	return c.JSON(http.StatusCreated, map[string]any{
		"id":        result.ID,
		"name":      result.Name,
		"parent_id": result.ParentID,
	})
}

// ListDepartments handles GET /api/workspaces/:id/departments.
func (h *AdminHandler) ListDepartments(c echo.Context) error {
	results, err := h.domain.ListDepartments(c.Request().Context(), c.Param("id"))
	if err != nil {
		return mapDomainError(err)
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
	var body struct {
		Name string `json:"name"`
	}
	if err := c.Bind(&body); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}

	result, err := h.domain.UpdateDepartment(c.Request().Context(), c.Param("deptId"), body.Name)
	if err != nil {
		return mapDomainError(err)
	}

	return c.JSON(http.StatusOK, map[string]any{
		"id":        result.ID,
		"name":      result.Name,
		"parent_id": result.ParentID,
	})
}

// DeleteDepartment handles DELETE /api/workspaces/:id/departments/:deptId.
func (h *AdminHandler) DeleteDepartment(c echo.Context) error {
	if err := h.domain.DeleteDepartment(c.Request().Context(), c.Param("deptId")); err != nil {
		return mapDomainError(err)
	}
	return c.NoContent(http.StatusNoContent)
}

// MoveDepartment handles PUT /api/workspaces/:id/departments/:deptId/move.
func (h *AdminHandler) MoveDepartment(c echo.Context) error {
	var body struct {
		NewParentID string `json:"new_parent_id"`
	}
	if err := c.Bind(&body); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}

	result, err := h.domain.MoveDepartment(c.Request().Context(), domain.MoveDepartmentInput{
		DeptID:      c.Param("deptId"),
		NewParentID: body.NewParentID,
	})
	if err != nil {
		return mapDomainError(err)
	}

	return c.JSON(http.StatusOK, map[string]any{
		"id":        result.ID,
		"name":      result.Name,
		"parent_id": result.ParentID,
	})
}

// UpdateMemberDepartment handles PUT /api/workspaces/:id/members/:nodeId/department.
func (h *AdminHandler) UpdateMemberDepartment(c echo.Context) error {
	var body struct {
		DepartmentID string `json:"department_id"`
	}
	if err := c.Bind(&body); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}

	if err := h.domain.UpdateMemberDepartment(
		c.Request().Context(),
		c.Param("id"),
		c.Param("nodeId"),
		body.DepartmentID,
	); err != nil {
		return mapDomainError(err)
	}

	return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
}

// mapDomainError translates domain errors to HTTP errors.
func mapDomainError(err error) *echo.HTTPError {
	switch {
	case domain.IsNotFound(err):
		return echo.NewHTTPError(http.StatusNotFound, err.Error())
	case domain.IsInvalidInput(err):
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	case domain.IsAlreadyExists(err):
		return echo.NewHTTPError(http.StatusConflict, err.Error())
	case domain.IsAccessDenied(err):
		return echo.NewHTTPError(http.StatusForbidden, err.Error())
	default:
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
}
