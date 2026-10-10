package rest

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"

	"ngac-platform/ngac"
	"ngac-platform/pkg/httputil"
	authpb "ngac-platform/proto/auth"
	policypb "ngac-platform/proto/policy"
	"ngac-platform/services/messaging/internal/domain"
	"ngac-platform/services/messaging/internal/store"
	"ngac-platform/testutil"
)

// answerPolicy decides every check the same way, so a test can show that the
// handlers turn a refusal into 403 and a grant into success.
type answerPolicy struct {
	policypb.PolicyReadServiceClient
	allow bool
}

func (a answerPolicy) CheckAccess(context.Context, *policypb.CheckAccessRequest, ...grpc.CallOption) (*policypb.AccessDecision, error) {
	if a.allow {
		return &policypb.AccessDecision{Decision: ngac.DecisionAllow}, nil
	}
	return &policypb.AccessDecision{Decision: ngac.DecisionDeny}, nil
}

type namedUser struct{ authpb.AuthServiceClient }

func (namedUser) GetUserByID(_ context.Context, r *authpb.GetUserByIDRequest, _ ...grpc.CallOption) (*authpb.UserInfo, error) {
	return &authpb.UserInfo{Id: r.UserId, Username: "tester"}, nil
}

type chat struct {
	pool  *pgxpool.Pool
	user  string
	node  string
	chID  string
	msgID string
}

func newChat(t *testing.T) *chat {
	t.Helper()
	pool := testutil.SetupTestDB(t)
	ctx := context.Background()
	user, node := testutil.CreateUser(t, pool)
	ws, _ := testutil.CreateWorkspace(t, pool, user)
	n := time.Now().UnixNano()
	c := &chat{pool: pool, user: user, node: node, chID: fmt.Sprintf("rest-ch-%d", n), msgID: fmt.Sprintf("rest-msg-%d", n)}
	oa := fmt.Sprintf("rest-oa-%d", n)
	_, err := pool.Exec(ctx, `INSERT INTO ngac_nodes (id, name, node_type) VALUES ($1, $2, $3)`, oa, "rest-oa-"+oa, ngac.TypeOA)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `INSERT INTO channels (id, name, channel_type, workspace_id, ngac_oa_id, created_at) VALUES ($1, 'rest', 'workspace', $2, $3, now())`, c.chID, ws, oa)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `INSERT INTO messages (id, channel_id, sender_id, content, content_format, message_type, created_at) VALUES ($1, $2, $3, 'findable text', 'plain', 'user', now())`, c.msgID, c.chID, user)
	require.NoError(t, err)
	t.Cleanup(func() {
		pool.Exec(ctx, `DELETE FROM message_pins WHERE channel_id = $1`, c.chID)
		pool.Exec(ctx, `DELETE FROM polls WHERE channel_id = $1`, c.chID)
		pool.Exec(ctx, `DELETE FROM chat_tasks WHERE channel_id = $1`, c.chID)
		pool.Exec(ctx, `DELETE FROM read_receipts WHERE channel_id = $1`, c.chID)
		pool.Exec(ctx, `DELETE FROM messages WHERE channel_id = $1`, c.chID)
		pool.Exec(ctx, `DELETE FROM channels WHERE id = $1`, c.chID)
		pool.Exec(ctx, `DELETE FROM ngac_nodes WHERE id = $1`, oa)
	})
	return c
}

func (c *chat) echo(allow bool) *echo.Echo {
	svc := domain.NewService(store.NewStore(c.pool), answerPolicy{allow: allow}, nil, namedUser{}, nil)
	h := NewHandler(svc, nil, nil)
	e := httputil.NewEcho("messaging")
	as := func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(ctx echo.Context) error {
			httputil.SetClaims(ctx, &httputil.Claims{UserID: c.user, NGACNodeID: c.node})
			return next(ctx)
		}
	}
	e.POST("/api/messages/:msgId/reactions", h.AddReaction, as)
	e.GET("/api/messages/:msgId/reactions", h.ListReactions, as)
	e.DELETE("/api/messages/:msgId/reactions/:emoji", h.RemoveReaction, as)
	e.POST("/api/channels/:chId/pins", h.PinMessage, as)
	e.GET("/api/channels/:chId/pins", h.ListPins, as)
	e.DELETE("/api/channels/:chId/pins/:msgId", h.UnpinMessage, as)
	e.POST("/api/channels/:chId/read", h.MarkChannelRead, as)
	e.GET("/api/channels/unread", h.GetUnreadCounts, as)
	e.GET("/api/channels/:chId/search", h.SearchMessages, as)
	e.POST("/api/channels/:chId/polls", h.CreatePoll, as)
	e.POST("/api/polls/:pollId/vote", h.VotePoll, as)
	e.DELETE("/api/polls/:pollId/vote", h.RemoveVote, as)
	e.GET("/api/polls/:pollId", h.GetPoll, as)
	e.POST("/api/channels/:chId/tasks", h.CreateTask, as)
	e.PATCH("/api/tasks/:taskId", h.UpdateTask, as)
	e.GET("/api/channels/:chId/tasks", h.ListTasks, as)
	e.GET("/api/channels/:chId/messages", h.GetMessages, as)
	e.GET("/api/messages/:msgId/thread", h.GetThread, as)
	e.GET("/api/channels/:chId", h.GetChannel, as)
	return e
}

func call(e *echo.Echo, method, path, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	return rec
}

func decode(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var out map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out), rec.Body.String())
	return out
}

func TestEngagementRoutes_AnswerForbiddenWhenPolicyRefuses(t *testing.T) {
	c := newChat(t)
	e := c.echo(false)
	for name, tc := range map[string]struct{ method, path, body string }{
		"add reaction":    {http.MethodPost, "/api/messages/" + c.msgID + "/reactions", `{"emoji":"x"}`},
		"list reactions":  {http.MethodGet, "/api/messages/" + c.msgID + "/reactions", ``},
		"remove reaction": {http.MethodDelete, "/api/messages/" + c.msgID + "/reactions/x", ``},
		"pin":             {http.MethodPost, "/api/channels/" + c.chID + "/pins", `{"message_id":"` + c.msgID + `"}`},
		"list pins":       {http.MethodGet, "/api/channels/" + c.chID + "/pins", ``},
		"unpin":           {http.MethodDelete, "/api/channels/" + c.chID + "/pins/" + c.msgID, ``},
		"mark read":       {http.MethodPost, "/api/channels/" + c.chID + "/read", `{"last_message_id":"` + c.msgID + `"}`},
		"search":          {http.MethodGet, "/api/channels/" + c.chID + "/search?q=findable", ``},
		"create poll":     {http.MethodPost, "/api/channels/" + c.chID + "/polls", `{"question":"q","options":["a","b"]}`},
		"create task":     {http.MethodPost, "/api/channels/" + c.chID + "/tasks", `{"title":"t"}`},
		"list tasks":      {http.MethodGet, "/api/channels/" + c.chID + "/tasks", ``},
		"messages":        {http.MethodGet, "/api/channels/" + c.chID + "/messages", ``},
		"thread":          {http.MethodGet, "/api/messages/" + c.msgID + "/thread", ``},
		"channel":         {http.MethodGet, "/api/channels/" + c.chID, ``},
	} {
		rec := call(e, tc.method, tc.path, tc.body)
		assert.Equal(t, http.StatusForbidden, rec.Code, "%s: %s", name, rec.Body)
	}
	var n int
	require.NoError(t, c.pool.QueryRow(context.Background(), `SELECT count(*) FROM message_pins WHERE channel_id = $1`, c.chID).Scan(&n))
	assert.Zero(t, n, "a refused pin wrote nothing")
}

func TestEngagementRoutes_BadBodiesAreRefusedBeforeAnyWork(t *testing.T) {
	c := newChat(t)
	e := c.echo(true)
	for name, tc := range map[string]struct{ method, path, body string }{
		"reaction json":  {http.MethodPost, "/api/messages/" + c.msgID + "/reactions", `{`},
		"pin json":       {http.MethodPost, "/api/channels/" + c.chID + "/pins", `{`},
		"read json":      {http.MethodPost, "/api/channels/" + c.chID + "/read", `{`},
		"poll json":      {http.MethodPost, "/api/channels/" + c.chID + "/polls", `{`},
		"vote json":      {http.MethodPost, "/api/polls/p/vote", `{`},
		"unvote json":    {http.MethodDelete, "/api/polls/p/vote", `{`},
		"task json":      {http.MethodPost, "/api/channels/" + c.chID + "/tasks", `{`},
		"task patch":     {http.MethodPatch, "/api/tasks/t", `{`},
		"reaction empty": {http.MethodPost, "/api/messages/" + c.msgID + "/reactions", `{"emoji":""}`},
		"poll one opt":   {http.MethodPost, "/api/channels/" + c.chID + "/polls", `{"question":"q","options":["a"]}`},
		"task no title":  {http.MethodPost, "/api/channels/" + c.chID + "/tasks", `{}`},
	} {
		rec := call(e, tc.method, tc.path, tc.body)
		assert.Equal(t, http.StatusBadRequest, rec.Code, "%s: %s", name, rec.Body)
	}
}

func TestEngagementRoutes_HappyPath(t *testing.T) {
	c := newChat(t)
	e := c.echo(true)

	require.Equal(t, http.StatusOK, call(e, http.MethodPost, "/api/messages/"+c.msgID+"/reactions", `{"emoji":"👍"}`).Code)
	rec := call(e, http.MethodGet, "/api/messages/"+c.msgID+"/reactions", "")
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Len(t, decode(t, rec)["reactions"], 1)
	require.Equal(t, http.StatusOK, call(e, http.MethodDelete, "/api/messages/"+c.msgID+"/reactions/%F0%9F%91%8D", "").Code)

	require.Equal(t, http.StatusOK, call(e, http.MethodPost, "/api/channels/"+c.chID+"/pins", `{"message_id":"`+c.msgID+`"}`).Code)
	rec = call(e, http.MethodGet, "/api/channels/"+c.chID+"/pins", "")
	assert.Len(t, decode(t, rec)["pins"], 1)
	require.Equal(t, http.StatusOK, call(e, http.MethodDelete, "/api/channels/"+c.chID+"/pins/"+c.msgID, "").Code)

	require.Equal(t, http.StatusOK, call(e, http.MethodPost, "/api/channels/"+c.chID+"/read", `{"last_message_id":"`+c.msgID+`"}`).Code)
	require.Equal(t, http.StatusOK, call(e, http.MethodGet, "/api/channels/unread", "").Code)

	rec = call(e, http.MethodGet, "/api/channels/"+c.chID+"/search?q=findable&limit=5", "")
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Len(t, decode(t, rec)["messages"], 1)

	rec = call(e, http.MethodPost, "/api/channels/"+c.chID+"/polls", `{"question":"lunch?","options":["pho","bun"]}`)
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	poll := decode(t, rec)
	pollID := poll["id"].(string)
	opt := poll["options"].([]any)[0].(map[string]any)["id"].(string)
	vote := `{"option_id":"` + opt + `"}`
	require.Equal(t, http.StatusOK, call(e, http.MethodPost, "/api/polls/"+pollID+"/vote", vote).Code)
	require.Equal(t, http.StatusOK, call(e, http.MethodGet, "/api/polls/"+pollID, "").Code)
	require.Equal(t, http.StatusOK, call(e, http.MethodDelete, "/api/polls/"+pollID+"/vote", vote).Code)
	assert.Equal(t, http.StatusBadRequest, call(e, http.MethodPost, "/api/polls/"+pollID+"/vote", `{"option_id":"not-an-option"}`).Code)

	rec = call(e, http.MethodPost, "/api/channels/"+c.chID+"/tasks", `{"title":"ship"}`)
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	taskID := decode(t, rec)["id"].(string)
	require.Equal(t, http.StatusOK, call(e, http.MethodPatch, "/api/tasks/"+taskID, `{"status":"done"}`).Code)
	rec = call(e, http.MethodGet, "/api/channels/"+c.chID+"/tasks?status=done", "")
	require.Equal(t, http.StatusOK, rec.Code)

	assert.Equal(t, http.StatusNotFound, call(e, http.MethodGet, "/api/polls/no-such-poll", "").Code)
	assert.Equal(t, http.StatusNotFound, call(e, http.MethodPatch, "/api/tasks/no-such-task", `{"status":"done"}`).Code)
}
