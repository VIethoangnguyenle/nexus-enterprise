package rest

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/labstack/echo/v4"
	echomw "github.com/labstack/echo/v4/middleware"

	"ngac-platform/pkg/httputil"
	"ngac-platform/services/document/internal/texts"
)

// TextService is what the text-document routes need. The caller is always the
// one in the verified JWT claims, never a field of the request.
type TextService interface {
	Create(ctx context.Context, who texts.Caller, workspaceID, folderID, title string) (*texts.Doc, error)
	Get(ctx context.Context, who texts.Caller, id string) (*texts.Opened, error)
	List(ctx context.Context, who texts.Caller, workspaceID, scope, cursor string, limit int) (*texts.Page, error)
	Count(ctx context.Context, who texts.Caller, workspaceID, scope string) (int, error)
	Update(ctx context.Context, who texts.Caller, id string, ch texts.Change) (*texts.Opened, error)
	Delete(ctx context.Context, who texts.Caller, id string) error
}

// A document is at most 1 MiB of content; the body limit leaves room for the
// JSON around it, and stops a larger request before it is read into memory.
const textBodyLimit = "2M"

type textHandler struct{ svc TextService }

func (h *Handler) registerTextRoutes(api *echo.Group) {
	if h.texts == nil {
		return
	}
	t := &textHandler{svc: h.texts}
	limit := echomw.BodyLimit(textBodyLimit)
	api.GET("/workspaces/:id/documents/texts", t.list)
	api.GET("/workspaces/:id/documents/texts/count", t.count)
	api.POST("/workspaces/:id/documents/texts", t.create, limit)
	api.GET("/documents/texts/:docId", t.get)
	api.PATCH("/documents/texts/:docId", t.update, limit)
	api.DELETE("/documents/texts/:docId", t.remove)
}

// textJSON is a document as clients see it. Ids are for calling the API and
// choosing a colour; the screen shows names.
type textJSON struct {
	ID          string    `json:"id"`
	WorkspaceID string    `json:"workspace_id"`
	FolderID    string    `json:"folder_id,omitempty"`
	Title       string    `json:"title"`
	Content     *string   `json:"content,omitempty"`
	Version     int       `json:"version"`
	Status      string    `json:"status"`
	OwnerID     string    `json:"owner_id"`
	OwnerName   string    `json:"owner_name"`
	CanWrite    *bool     `json:"can_write,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func toJSON(d *texts.Doc, withContent bool, canWrite *bool) textJSON {
	out := textJSON{
		ID: d.ID, WorkspaceID: d.WorkspaceID, FolderID: d.FolderID, Title: d.Title, Version: d.Version,
		Status: d.Status, OwnerID: d.OwnerID, OwnerName: d.OwnerName, CanWrite: canWrite,
		CreatedAt: d.CreatedAt, UpdatedAt: d.UpdatedAt,
	}
	if withContent {
		c := d.Content
		out.Content = &c
	}
	return out
}

func callerOf(c echo.Context) (texts.Caller, error) {
	claims, err := httputil.RequireClaims(c)
	if err != nil {
		return texts.Caller{}, err
	}
	return texts.Caller{UserID: claims.UserID, NGACNodeID: claims.NGACNodeID, TenantID: claims.TenantID}, nil
}

func (t *textHandler) list(c echo.Context) error {
	who, err := callerOf(c)
	if err != nil {
		return err
	}
	size := 0
	if v := c.QueryParam("limit"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 {
			return echo.NewHTTPError(http.StatusBadRequest, "limit must be a positive number")
		}
		size = n
	}
	page, err := t.svc.List(c.Request().Context(), who, c.Param("id"), c.QueryParam("scope"), c.QueryParam("cursor"), size)
	if err != nil {
		return textError(c, err)
	}
	out := make([]textJSON, len(page.Docs))
	for i, d := range page.Docs {
		out[i] = toJSON(d.Doc, false, &d.CanWrite)
	}
	return c.JSON(http.StatusOK, map[string]any{"documents": out, "next_cursor": page.Next})
}

// count handles GET /workspaces/:id/documents/texts/count: how many documents of
// a listing the caller may read, for a badge, without loading them.
func (t *textHandler) count(c echo.Context) error {
	who, err := callerOf(c)
	if err != nil {
		return err
	}
	n, err := t.svc.Count(c.Request().Context(), who, c.Param("id"), c.QueryParam("scope"))
	if err != nil {
		return textError(c, err)
	}
	return c.JSON(http.StatusOK, map[string]int{"count": n})
}

func (t *textHandler) create(c echo.Context) error {
	who, err := callerOf(c)
	if err != nil {
		return err
	}
	var body struct {
		Title    string `json:"title"`
		FolderID string `json:"folder_id"`
	}
	if err := c.Bind(&body); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}
	d, err := t.svc.Create(c.Request().Context(), who, c.Param("id"), body.FolderID, body.Title)
	if err != nil {
		return textError(c, err)
	}
	yes := true
	return c.JSON(http.StatusCreated, toJSON(d, true, &yes))
}

func (t *textHandler) get(c echo.Context) error {
	who, err := callerOf(c)
	if err != nil {
		return err
	}
	o, err := t.svc.Get(c.Request().Context(), who, c.Param("docId"))
	if err != nil {
		return textError(c, err)
	}
	return c.JSON(http.StatusOK, toJSON(o.Doc, true, &o.CanWrite))
}

func (t *textHandler) update(c echo.Context) error {
	who, err := callerOf(c)
	if err != nil {
		return err
	}
	var body struct {
		BaseVersion int     `json:"base_version"`
		Title       *string `json:"title"`
		Content     *string `json:"content"`
		Status      *string `json:"status"`
	}
	if err := c.Bind(&body); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}
	o, err := t.svc.Update(c.Request().Context(), who, c.Param("docId"), texts.Change{
		BaseVersion: body.BaseVersion, Title: body.Title, Content: body.Content, Status: body.Status,
	})
	if err != nil {
		return textError(c, err)
	}
	return c.JSON(http.StatusOK, toJSON(o.Doc, o.CanRead, &o.CanWrite))
}

func (t *textHandler) remove(c echo.Context) error {
	who, err := callerOf(c)
	if err != nil {
		return err
	}
	if err := t.svc.Delete(c.Request().Context(), who, c.Param("docId")); err != nil {
		return textError(c, err)
	}
	return c.NoContent(http.StatusNoContent)
}

// textError turns a service error into a response. A conflict carries the
// document as it now stands so the client can offer to reload or compare;
// anything unexpected is logged and answered with a generic body, never the
// error text (which may come from the database).
func textError(c echo.Context, err error) error {
	var conflict *texts.ConflictError
	switch {
	case errors.As(err, &conflict):
		body := map[string]any{"error": "Văn bản đã được sửa ở nơi khác", "reason": "version_conflict"}
		if conflict.Readable {
			yes := true
			body["current"] = toJSON(conflict.Current, true, &yes)
		} else {
			// Not allowed to read it: the version is all there is to say.
			body["current"] = map[string]any{"version": conflict.Current.Version}
		}
		return c.JSON(http.StatusConflict, body)
	case errors.Is(err, httputil.ErrAccessDenied):
		return echo.NewHTTPError(http.StatusForbidden, "access denied")
	case errors.Is(err, httputil.ErrInvalidInput):
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	slog.Error("text document request failed", "path", c.Path(), "error", err)
	return echo.NewHTTPError(http.StatusInternalServerError, "internal error")
}
