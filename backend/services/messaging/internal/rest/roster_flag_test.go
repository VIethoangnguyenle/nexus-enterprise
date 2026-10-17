package rest

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"

	"ngac-platform/ngac"
	"ngac-platform/pkg/httputil"
	policypb "ngac-platform/proto/policy"
	"ngac-platform/services/messaging/internal/domain"
	"ngac-platform/services/messaging/internal/store"
	"ngac-platform/testutil"
)

// readOnlyPolicy lets everyone read and nothing else.
type readOnlyPolicy struct {
	policypb.PolicyReadServiceClient
}

func (readOnlyPolicy) CheckAccess(_ context.Context, r *policypb.CheckAccessRequest, _ ...grpc.CallOption) (*policypb.AccessDecision, error) {
	if r.Operation == ngac.OpRead {
		return &policypb.AccessDecision{Decision: ngac.DecisionAllow}, nil
	}
	return &policypb.AccessDecision{Decision: ngac.DecisionDeny}, nil
}

func (readOnlyPolicy) GetChildren(context.Context, *policypb.GetChildrenRequest, ...grpc.CallOption) (*policypb.NodeList, error) {
	return &policypb.NodeList{}, nil
}

// The members list says, in words a screen can branch on, whether the reader may
// remove people: the group's creator may, another member may not.
func TestListChannelMembers_TellsWhoMayRemove(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	ctx := context.Background()
	creator, _ := testutil.CreateUser(t, pool)
	other, _ := testutil.CreateUser(t, pool)
	oa, ua := "oa-"+uuid.NewString(), "ua-"+uuid.NewString()
	for _, n := range [][2]string{{oa, "OA"}, {ua, "UA"}} {
		_, err := pool.Exec(ctx, `INSERT INTO ngac_nodes (id, name, node_type) VALUES ($1, $1, $2)`, n[0], n[1])
		require.NoError(t, err)
	}
	chID := "ch-" + uuid.NewString()
	st := store.NewStore(pool)
	require.NoError(t, st.InsertChannelWithMembers(ctx, &store.Channel{ID: chID, Name: "roster-flag", ChannelType: "private", NGACOaID: oa, NGACUaID: ua, CreatedBy: creator}, "creator-node"))
	t.Cleanup(func() {
		pool.Exec(ctx, `DELETE FROM channels WHERE id = $1`, chID)
		pool.Exec(ctx, `DELETE FROM ngac_nodes WHERE id IN ($1, $2)`, oa, ua)
	})
	h := NewHandler(domain.NewService(st, readOnlyPolicy{}, nil, nil, nil), nil, nil)

	flag := func(userID, node string) bool {
		e := httputil.NewEcho("messaging")
		e.GET("/api/channels/:chId/members", h.ListChannelMembers, func(next echo.HandlerFunc) echo.HandlerFunc {
			return func(c echo.Context) error {
				httputil.SetClaims(c, &httputil.Claims{UserID: userID, NGACNodeID: node})
				return next(c)
			}
		})
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/channels/"+chID+"/members", nil))
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		var body struct {
			CanRemove *bool `json:"can_remove_members"`
		}
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
		require.NotNil(t, body.CanRemove, "the flag is always present: "+rec.Body.String())
		return *body.CanRemove
	}

	assert.False(t, flag(other, "other-node"), "a member who did not make the group")
	assert.True(t, flag(creator, "creator-node"), "the group's creator")
}
