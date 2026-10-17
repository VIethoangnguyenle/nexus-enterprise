package rest

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"

	"ngac-platform/ngac"
	"ngac-platform/pkg/httputil"
	policypb "ngac-platform/proto/policy"
	"ngac-platform/services/messaging/internal/domain"
	"ngac-platform/services/messaging/internal/store"
)

// channelsPolicy answers every check with allow and resolves the workspace's
// Channels OA, so channel creation gets past its create_channel check.
type channelsPolicy struct {
	answerPolicy
	ws string
}

func (p channelsPolicy) GetChildren(context.Context, *policypb.GetChildrenRequest, ...grpc.CallOption) (*policypb.NodeList, error) {
	return &policypb.NodeList{Nodes: []*policypb.NGACNode{
		{Id: "rest-channels-oa-" + p.ws, Name: ngac.ChannelsOAName(ngac.WorkspaceID(p.ws)), NodeType: ngac.TypeOA},
	}}, nil
}

// persistingWrite writes the nodes a channel row has foreign keys onto.
type persistingWrite struct {
	policypb.PolicyWriteServiceClient
	pool *pgxpool.Pool
}

func (w persistingWrite) CreateNode(ctx context.Context, r *policypb.CreateNodeRequest, _ ...grpc.CallOption) (*policypb.NGACNode, error) {
	id := "rest-node-" + r.Name
	if _, err := w.pool.Exec(ctx, `INSERT INTO ngac_nodes (id, name, node_type) VALUES ($1, $2, $3) ON CONFLICT (id) DO NOTHING`, id, r.Name, r.NodeType); err != nil {
		return nil, err
	}
	return &policypb.NGACNode{Id: id, Name: r.Name, NodeType: r.NodeType}, nil
}
func (w persistingWrite) DeleteNode(ctx context.Context, r *policypb.DeleteNodeRequest, _ ...grpc.CallOption) (*policypb.Empty, error) {
	_, err := w.pool.Exec(ctx, `DELETE FROM ngac_nodes WHERE id = $1`, r.NodeId)
	return &policypb.Empty{}, err
}
func (persistingWrite) CreateAssignment(context.Context, *policypb.CreateAssignmentRequest, ...grpc.CallOption) (*policypb.Assignment, error) {
	return &policypb.Assignment{Id: "a"}, nil
}
func (persistingWrite) CreateAssociation(context.Context, *policypb.CreateAssociationRequest, ...grpc.CallOption) (*policypb.Association, error) {
	return &policypb.Association{Id: "a"}, nil
}

// POST /workspaces/:id/channels makes a private group whatever channel_type the
// body names: the workspace-wide channel is made by the auth service alone, so a
// body that says "workspace" (what the client sent before groups were private)
// or anything else must not be able to ask for it.
func TestCreateChannelRoute_AlwaysStoresPrivate(t *testing.T) {
	for _, sent := range []string{"workspace", "group", "private", "", "dm", "no-such-type"} {
		t.Run(fmt.Sprintf("body %q", sent), func(t *testing.T) {
			c := newChat(t)
			svc := domain.NewService(store.NewStore(c.pool), channelsPolicy{answerPolicy: answerPolicy{allow: true}, ws: c.ws},
				persistingWrite{pool: c.pool}, namedUser{}, nil)
			h := NewHandler(svc, nil, nil)
			e := httputil.NewEcho("messaging")
			e.POST("/api/workspaces/:id/channels", h.CreateChannel, func(next echo.HandlerFunc) echo.HandlerFunc {
				return func(ctx echo.Context) error {
					httputil.SetClaims(ctx, &httputil.Claims{UserID: c.user, NGACNodeID: c.node, TenantID: c.ws})
					return next(ctx)
				}
			})
			body := fmt.Sprintf(`{"name":"rest-private","channel_type":%q}`, sent)
			req := httptest.NewRequest(http.MethodPost, "/api/workspaces/"+c.ws+"/channels", strings.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()

			e.ServeHTTP(rec, req)

			require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
			var ch struct {
				Id          string `json:"id"`
				ChannelType string `json:"channel_type"`
				NgacOaId    string `json:"ngac_oa_id"`
				NgacUaId    string `json:"ngac_ua_id"`
			}
			require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &ch))
			t.Cleanup(func() {
				c.pool.Exec(context.Background(), `DELETE FROM channels WHERE id = $1`, ch.Id)
				c.pool.Exec(context.Background(), `DELETE FROM ngac_nodes WHERE id IN ($1, $2)`, ch.NgacOaId, ch.NgacUaId)
			})
			assert.Equal(t, "private", ch.ChannelType)
			var stored string
			require.NoError(t, c.pool.QueryRow(context.Background(), `SELECT channel_type FROM channels WHERE id = $1`, ch.Id).Scan(&stored))
			assert.Equal(t, "private", stored)
		})
	}
}
