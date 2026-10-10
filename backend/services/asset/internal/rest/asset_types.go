// Package rest provides Echo REST handlers for the asset service. They adapt
// HTTP to the asset domain services and hold no business logic.
package rest

import (
	"net/http"

	"github.com/labstack/echo/v4"

	pb "ngac-platform/proto/asset"
)

func (h *Handler) CreateAssetType(c echo.Context) error {
	var body struct {
		Name         string `json:"name"`
		Category     string `json:"category"`
		Description  string `json:"description"`
		FieldsSchema string `json:"fields_schema"`
	}
	if err := c.Bind(&body); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}
	resp, err := h.typeSvc.CreateType(c.Request().Context(), &pb.CreateTypeRequest{
		WorkspaceId:  c.Param("id"),
		Name:         body.Name,
		Category:     body.Category,
		Description:  body.Description,
		FieldsSchema: body.FieldsSchema,
	})
	if err != nil {
		return mapError(err)
	}
	return c.JSON(http.StatusCreated, resp)
}

func (h *Handler) ListAssetTypes(c echo.Context) error {
	resp, err := h.typeSvc.ListTypes(c.Request().Context(), &pb.ListTypesRequest{
		WorkspaceId: c.Param("id"),
	})
	if err != nil {
		return mapError(err)
	}
	return c.JSON(http.StatusOK, resp)
}

func (h *Handler) GetAssetType(c echo.Context) error {
	resp, err := h.typeSvc.GetType(c.Request().Context(), &pb.GetTypeRequest{
		TypeId: c.Param("typeId"),
	})
	if err != nil {
		return mapError(err)
	}
	return c.JSON(http.StatusOK, resp)
}

func (h *Handler) UpdateAssetTypeSchema(c echo.Context) error {
	var body struct {
		FieldsSchema string `json:"fields_schema"`
	}
	if err := c.Bind(&body); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}
	resp, err := h.typeSvc.UpdateTypeSchema(c.Request().Context(), &pb.UpdateTypeSchemaRequest{
		TypeId:       c.Param("typeId"),
		FieldsSchema: body.FieldsSchema,
	})
	if err != nil {
		return mapError(err)
	}
	return c.JSON(http.StatusOK, resp)
}
