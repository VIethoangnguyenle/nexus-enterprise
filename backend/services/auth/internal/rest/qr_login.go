// Package rest — sign-in by QR code.
package rest

import (
	"net/http"
	"time"

	"github.com/labstack/echo/v4"

	"ngac-platform/pkg/httputil"
	"ngac-platform/services/auth/internal/domain"
)

func (h *Handler) registerQRRoutes(e *echo.Echo, api *echo.Group) {
	// The computer, not signed in.
	e.POST("/api/auth/qr", h.CreateQRLogin, h.publicLimiter("qr-create"))
	// The page polls every two seconds and an office signs in from one address,
	// so the status route gets five times the allowance of the sign-in routes.
	statusLimit := h.publicLimit()
	statusLimit.Max *= qrStatusAllowance
	e.POST("/api/auth/qr/status", h.QRLoginStatus, h.limiterAt("qr-status", statusLimit))
	// The phone, signed in.
	api.POST("/auth/qr/scan", h.ScanQRLogin)
	api.POST("/auth/qr/approve", h.ApproveQRLogin)
	api.POST("/auth/qr/decline", h.DeclineQRLogin)
}

// qrStatusAllowance multiplies the public per-address limit for the status route.
const qrStatusAllowance = 5

// qrBody is the body of every QR route but create: the code's token, and on the
// status route also its poll secret. Both travel in bodies, never in a URL.
type qrBody struct {
	Token      string `json:"token"`
	PollSecret string `json:"poll_secret"`
}

func qrTokenBody(c echo.Context) (string, error) {
	var body qrBody
	if err := c.Bind(&body); err != nil {
		return "", badBody()
	}
	return body.Token, nil
}

// CreateQRLogin handles POST /api/auth/qr: a new code for this browser.
func (h *Handler) CreateQRLogin(c echo.Context) error {
	ticket, err := h.svc.CreateQRLogin(c.Request().Context(), c.Request().UserAgent())
	if err != nil {
		return fail(c, err)
	}
	// The poll secret goes only to the page that asked, which keeps it in memory:
	// the QR shows the token alone, so whoever sees the screen cannot collect the
	// session. One secret per code, so two sign-in tabs never disturb each other.
	return c.JSON(http.StatusOK, map[string]any{
		"token": ticket.Token, "poll_secret": ticket.PollSecret, "expires_in": int(ticket.ExpiresIn / time.Second),
	})
}

// QRLoginStatus handles POST /api/auth/qr/status: how this browser's code is
// doing. Once the phone approved, this is the response that signs the browser
// in, with the same body and refresh cookie as a verified sign-in code.
func (h *Handler) QRLoginStatus(c echo.Context) error {
	var in qrBody
	if err := c.Bind(&in); err != nil {
		return badBody()
	}
	poll, err := h.svc.PollQRLogin(c.Request().Context(), in.Token, in.PollSecret)
	if err != nil {
		return fail(c, err)
	}

	body := map[string]any{"status": poll.Status}
	if poll.Scanner != nil {
		body["scanner"] = map[string]string{"display_name": poll.Scanner.DisplayName, "avatar_url": poll.Scanner.AvatarURL}
	}
	if s := poll.Session; s != nil {
		if err := h.issueSession(c, domain.RefreshIdentity{
			UserID: s.UserID, Username: s.Username, NGACNodeID: s.NGACNodeID,
			TenantID: s.TenantID, SessionID: s.SessionID,
		}); err != nil {
			return err
		}
		body["access_token"] = s.Token
		body["user"] = map[string]any{
			"id": s.UserID, "username": s.Username,
			"ngac_node_id": s.NGACNodeID, "email": s.Email,
			"phone": s.Phone, "union_id": s.UnionID,
			"email_verified": s.EmailVerified,
		}
		body["needs_profile"] = s.NeedsProfile
	}
	return c.JSON(http.StatusOK, body)
}

func qrPhone(claims *httputil.Claims) domain.QRPhone {
	return domain.QRPhone{
		UserID: claims.UserID, Username: claims.Username, NGACNodeID: claims.NGACNodeID,
		TenantID: claims.TenantID, SessionID: claims.SessionID,
	}
}

// ScanQRLogin handles POST /api/auth/qr/scan: the phone opened a code and is
// told which computer is asking.
func (h *Handler) ScanQRLogin(c echo.Context) error {
	claims, err := httputil.RequireClaims(c)
	if err != nil {
		return err
	}
	token, err := qrTokenBody(c)
	if err != nil {
		return err
	}
	scan, err := h.svc.ScanQRLogin(c.Request().Context(), token, qrPhone(claims))
	if err != nil {
		return fail(c, err)
	}
	return c.JSON(http.StatusOK, map[string]any{
		"browser": scan.Browser, "os": scan.OS,
		"created_at": scan.CreatedAt, "tenant_name": scan.TenantName,
	})
}

// ApproveQRLogin handles POST /api/auth/qr/approve.
func (h *Handler) ApproveQRLogin(c echo.Context) error {
	claims, err := httputil.RequireClaims(c)
	if err != nil {
		return err
	}
	token, err := qrTokenBody(c)
	if err != nil {
		return err
	}
	if err := h.svc.ApproveQRLogin(c.Request().Context(), token, qrPhone(claims)); err != nil {
		return fail(c, err)
	}
	return c.NoContent(http.StatusNoContent)
}

// DeclineQRLogin handles POST /api/auth/qr/decline.
func (h *Handler) DeclineQRLogin(c echo.Context) error {
	claims, err := httputil.RequireClaims(c)
	if err != nil {
		return err
	}
	token, err := qrTokenBody(c)
	if err != nil {
		return err
	}
	if err := h.svc.DeclineQRLogin(c.Request().Context(), token, qrPhone(claims)); err != nil {
		return fail(c, err)
	}
	return c.NoContent(http.StatusNoContent)
}
