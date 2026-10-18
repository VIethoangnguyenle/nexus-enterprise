package rest

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"ngac-platform/pkg/httputil"
	policypb "ngac-platform/proto/policy"
	"ngac-platform/services/messaging/internal/domain"
	"ngac-platform/services/messaging/internal/store"
)

// strangerPolicy knows no user node: whoever asks is not a member of any
// workspace. nodeErr makes the node lookup itself fail.
type strangerPolicy struct {
	answerPolicy
	nodeErr error
}

func (p strangerPolicy) GetNode(context.Context, *policypb.GetNodeRequest, ...grpc.CallOption) (*policypb.NGACNode, error) {
	if p.nodeErr != nil {
		return nil, p.nodeErr
	}
	return nil, status.Error(codes.NotFound, "no such node")
}

func serveInCommon(t *testing.T, pol policypb.PolicyReadServiceClient, path string) *httptest.ResponseRecorder {
	t.Helper()
	c := newChat(t)
	svc := domain.NewService(store.NewStore(c.pool), pol, nil, namedUser{}, nil)
	h := NewHandler(svc, nil, nil)
	e := httputil.NewEcho("messaging")
	as := func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(ctx echo.Context) error {
			httputil.SetClaims(ctx, &httputil.Claims{UserID: c.user, NGACNodeID: c.node, TenantID: c.ws})
			return next(ctx)
		}
	}
	e.GET("/api/workspaces/:id/channels/in-common", h.GroupsInCommon, as)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	return rec
}

// Deny: a caller who is not a member of the workspace gets 403 and no channel.
func TestGroupsInCommonRoute_NonMemberIs403(t *testing.T) {
	rec := serveInCommon(t, strangerPolicy{answerPolicy: answerPolicy{allow: true}}, "/api/workspaces/ws-x/channels/in-common?with=ngac-someone")
	assert.Equal(t, http.StatusForbidden, rec.Code, rec.Body.String())
	assert.NotContains(t, rec.Body.String(), "channels")
}

// The person is required; without one the question is malformed.
func TestGroupsInCommonRoute_MissingPersonIs400(t *testing.T) {
	rec := serveInCommon(t, strangerPolicy{answerPolicy: answerPolicy{allow: true}}, "/api/workspaces/ws-x/channels/in-common")
	assert.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
}

// A membership that cannot be decided is an outage, never an answer.
func TestGroupsInCommonRoute_PolicyOutageIsNotA200(t *testing.T) {
	rec := serveInCommon(t, strangerPolicy{answerPolicy: answerPolicy{allow: true}, nodeErr: errors.New("policy down")},
		"/api/workspaces/ws-x/channels/in-common?with=ngac-someone")
	assert.GreaterOrEqual(t, rec.Code, http.StatusInternalServerError, rec.Body.String())
	assert.NotContains(t, rec.Body.String(), "policy down", "the cause stays in the log")
}
