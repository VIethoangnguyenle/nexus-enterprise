// Package rest — polls and tasks REST handlers.
package rest

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"ngac-platform/pkg/httputil"
	"ngac-platform/services/messaging/internal/domain"
)

// --- Polls ---

// CreatePoll handles POST /api/channels/:chId/polls.
func (h *Handler) CreatePoll(c echo.Context) error {
	claims := httputil.GetClaims(c)
	var body struct {
		Question    string   `json:"question"`
		Options     []string `json:"options"`
		IsMulti     bool     `json:"is_multi"`
		IsAnonymous bool     `json:"is_anonymous"`
		EndsAt      string   `json:"ends_at"`
	}
	if err := c.Bind(&body); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}

	poll, err := h.svc.CreatePoll(c.Request().Context(), domain.CreatePollInput{
		ChannelID:   c.Param("chId"),
		UserID:      claims.UserID,
		UserNodeID:  claims.NGACNodeID,
		Question:    body.Question,
		Options:     body.Options,
		IsMulti:     body.IsMulti,
		IsAnonymous: body.IsAnonymous,
	})
	if err != nil {
		return httputil.MapDomainError(err)
	}
	return c.JSON(http.StatusCreated, poll)
}

// VotePoll handles POST /api/polls/:pollId/vote.
func (h *Handler) VotePoll(c echo.Context) error {
	claims := httputil.GetClaims(c)
	var body struct {
		OptionID string `json:"option_id"`
	}
	if err := c.Bind(&body); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}

	if err := h.svc.VotePoll(c.Request().Context(), c.Param("pollId"), body.OptionID, claims.UserID); err != nil {
		return httputil.MapDomainError(err)
	}
	return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
}

// RemoveVote handles DELETE /api/polls/:pollId/vote.
func (h *Handler) RemoveVote(c echo.Context) error {
	claims := httputil.GetClaims(c)
	var body struct {
		OptionID string `json:"option_id"`
	}
	if err := c.Bind(&body); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}

	if err := h.svc.RemoveVote(c.Request().Context(), c.Param("pollId"), body.OptionID, claims.UserID); err != nil {
		return httputil.MapDomainError(err)
	}
	return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
}

// GetPoll handles GET /api/polls/:pollId.
func (h *Handler) GetPoll(c echo.Context) error {
	poll, err := h.svc.GetPoll(c.Request().Context(), c.Param("pollId"))
	if err != nil {
		return httputil.MapDomainError(err)
	}
	return c.JSON(http.StatusOK, poll)
}

// --- Tasks ---

// CreateTask handles POST /api/channels/:chId/tasks.
func (h *Handler) CreateTask(c echo.Context) error {
	claims := httputil.GetClaims(c)
	var body struct {
		Title      string `json:"title"`
		AssigneeID string `json:"assignee_id"`
		DueDate    string `json:"due_date"`
	}
	if err := c.Bind(&body); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}

	task, err := h.svc.CreateTask(c.Request().Context(), domain.CreateTaskInput{
		ChannelID:  c.Param("chId"),
		UserID:     claims.UserID,
		UserNodeID: claims.NGACNodeID,
		Title:      body.Title,
		AssigneeID: body.AssigneeID,
		DueDate:    body.DueDate,
	})
	if err != nil {
		return httputil.MapDomainError(err)
	}
	return c.JSON(http.StatusCreated, task)
}

// UpdateTask handles PATCH /api/tasks/:taskId.
func (h *Handler) UpdateTask(c echo.Context) error {
	claims := httputil.GetClaims(c)
	var body struct {
		Status     string `json:"status"`
		AssigneeID string `json:"assignee_id"`
		Title      string `json:"title"`
		DueDate    string `json:"due_date"`
	}
	if err := c.Bind(&body); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}

	task, err := h.svc.UpdateTask(c.Request().Context(), domain.UpdateTaskInput{
		TaskID:     c.Param("taskId"),
		UserID:     claims.UserID,
		Status:     body.Status,
		AssigneeID: body.AssigneeID,
		Title:      body.Title,
		DueDate:    body.DueDate,
	})
	if err != nil {
		return httputil.MapDomainError(err)
	}
	return c.JSON(http.StatusOK, task)
}

// ListTasks handles GET /api/channels/:chId/tasks.
func (h *Handler) ListTasks(c echo.Context) error {
	status := c.QueryParam("status")
	tasks, err := h.svc.ListTasks(c.Request().Context(), c.Param("chId"), status)
	if err != nil {
		return httputil.MapDomainError(err)
	}
	return c.JSON(http.StatusOK, map[string]any{"tasks": tasks})
}
