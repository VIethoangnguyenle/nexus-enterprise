package rest

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"ngac-platform/ngac"
	"ngac-platform/pkg/httputil"
	policypb "ngac-platform/proto/policy"
	"ngac-platform/services/messaging/internal/domain"
	"ngac-platform/services/messaging/internal/store"
	"ngac-platform/testutil"
)

// managersLookupWriter is a writer in whose graph everyone named is a person in
// the group, and the Managers UA lookup answers with findErr.
type managersLookupWriter struct {
	policypb.PolicyWriteServiceClient
	findErr  error
	assigned []string
}

func (w *managersLookupWriter) GetNode(_ context.Context, r *policypb.GetNodeRequest, _ ...grpc.CallOption) (*policypb.NGACNode, error) {
	return &policypb.NGACNode{Id: r.NodeId, NodeType: ngac.TypeU}, nil
}

func (w *managersLookupWriter) IsAssigned(context.Context, *policypb.IsAssignedRequest, ...grpc.CallOption) (*policypb.BoolResponse, error) {
	return &policypb.BoolResponse{Value: true}, nil
}

func (w *managersLookupWriter) FindNodeByName(_ context.Context, r *policypb.FindNodeByNameRequest, _ ...grpc.CallOption) (*policypb.NGACNode, error) {
	if w.findErr != nil {
		return nil, w.findErr
	}
	return &policypb.NGACNode{Id: "managers-ua", Name: r.Name, NodeType: r.NodeType}, nil
}

func (w *managersLookupWriter) CreateAssignment(_ context.Context, r *policypb.CreateAssignmentRequest, _ ...grpc.CallOption) (*policypb.Assignment, error) {
	w.assigned = append(w.assigned, r.ChildId+">"+r.ParentId)
	return &policypb.Assignment{}, nil
}

type groupREST struct {
	chID               string
	owner, ownerNode   string
	target, targetNode string
	serve              func(w *managersLookupWriter, method, path string) *httptest.ResponseRecorder
}

func newGroupREST(t *testing.T) *groupREST {
	t.Helper()
	pool := testutil.SetupTestDB(t)
	ctx := context.Background()
	g := &groupREST{}
	g.owner, g.ownerNode = testutil.CreateUser(t, pool)
	g.target, g.targetNode = testutil.CreateUser(t, pool)
	oa, ua := "oa-"+uuid.NewString(), "ua-"+uuid.NewString()
	for _, n := range [][2]string{{oa, "OA"}, {ua, "UA"}} {
		_, err := pool.Exec(ctx, `INSERT INTO ngac_nodes (id, name, node_type) VALUES ($1, $1, $2)`, n[0], n[1])
		require.NoError(t, err)
	}
	g.chID = "ch-" + uuid.NewString()
	st := store.NewStore(pool)
	ws, _ := testutil.CreateWorkspace(t, pool, g.owner)
	require.NoError(t, st.InsertChannelWithMembers(ctx, &store.Channel{
		ID: g.chID, Name: "roles", ChannelType: "private", WorkspaceID: ws, NGACOaID: oa, NGACUaID: ua, CreatedBy: g.owner,
	}, g.ownerNode))
	t.Cleanup(func() {
		pool.Exec(ctx, `DELETE FROM channels WHERE id = $1`, g.chID)
		pool.Exec(ctx, `DELETE FROM ngac_nodes WHERE id IN ($1, $2)`, oa, ua)
	})
	g.serve = func(w *managersLookupWriter, method, path string) *httptest.ResponseRecorder {
		h := NewHandler(domain.NewService(st, readOnlyPolicy{}, w, nil, nil), nil, nil)
		e := httputil.NewEcho("messaging")
		api := e.Group("/api", func(next echo.HandlerFunc) echo.HandlerFunc {
			return func(c echo.Context) error {
				httputil.SetClaims(c, &httputil.Claims{UserID: g.owner, NGACNodeID: g.ownerNode})
				return next(c)
			}
		})
		h.registerGroupManagementRoutes(api)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, httptest.NewRequest(method, path, nil))
		return rec
	}
	return g
}

// The appoint endpoint names the person by user ID and the server finds their
// node: the write is on the node, never on what the client typed.
func TestAppointManager_TakesAUserIDResolvedOnTheServer(t *testing.T) {
	g := newGroupREST(t)
	w := &managersLookupWriter{}

	rec := g.serve(w, http.MethodPost, "/api/channels/"+g.chID+"/managers/"+g.target)

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Equal(t, []string{g.targetNode + ">managers-ua"}, w.assigned)
}

func TestAppointManager_UnknownUserIsNotFoundAndNothingWritten(t *testing.T) {
	g := newGroupREST(t)
	w := &managersLookupWriter{}

	rec := g.serve(w, http.MethodPost, "/api/channels/"+g.chID+"/managers/no-such-user")

	assert.Equal(t, http.StatusNotFound, rec.Code, rec.Body.String())
	assert.Empty(t, w.assigned)
}

// Neither a missing Managers UA nor a failing policy service tells the client
// which channel or node it was about.
func TestGroupManagementErrors_DoNotCarryTheChannelID(t *testing.T) {
	g := newGroupREST(t)

	missing := g.serve(&managersLookupWriter{findErr: status.Error(codes.NotFound, "node Ch_"+g.chID+"_Managers not found")},
		http.MethodPost, "/api/channels/"+g.chID+"/managers/"+g.target)
	assert.Equal(t, http.StatusNotFound, missing.Code, missing.Body.String())
	assert.NotContains(t, missing.Body.String(), g.chID)

	down := g.serve(&managersLookupWriter{findErr: status.Error(codes.Unavailable, "policy down looking up Ch_"+g.chID+"_Managers")},
		http.MethodPost, "/api/channels/"+g.chID+"/managers/"+g.target)
	assert.Equal(t, http.StatusInternalServerError, down.Code, down.Body.String())
	assert.Contains(t, down.Body.String(), httputil.InternalMessage)
	assert.NotContains(t, down.Body.String(), g.chID)
}
