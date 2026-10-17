package rest

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"hash/crc32"
	"image"
	"image/color"
	"image/png"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/redis/go-redis/v9"
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

func pngBytes(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{R: uint8(x), G: uint8(y), B: 40, A: 255})
		}
	}
	var buf bytes.Buffer
	require.NoError(t, png.Encode(&buf, img))
	return buf.Bytes()
}

// avatarEcho serves the picture routes as the chat's user, with policy
// answering every check with allow.
func (c *chat) avatarEcho(allow bool, rdb *redis.Client) *echo.Echo {
	return c.avatarEchoWith(answerPolicy{allow: allow}, rdb)
}

func (c *chat) avatarEchoWith(policy policypb.PolicyReadServiceClient, rdb *redis.Client) *echo.Echo {
	svc := domain.NewService(store.NewStore(c.pool), policy, nil, namedUser{}, nil)
	if rdb != nil {
		svc.SetRedis(rdb)
	}
	h := NewHandler(svc, nil, nil)
	e := httputil.NewEcho("messaging")
	api := e.Group("/api", func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(ctx echo.Context) error {
			httputil.SetClaims(ctx, &httputil.Claims{UserID: c.user, NGACNodeID: c.node, TenantID: c.ws})
			return next(ctx)
		}
	})
	h.registerAvatarRoutes(e, api)
	return e
}

func upload(t *testing.T, e *echo.Echo, path, field, filename string, body []byte) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, err := mw.CreateFormFile(field, filename)
	require.NoError(t, err)
	_, err = fw.Write(body)
	require.NoError(t, err)
	require.NoError(t, mw.Close())
	req := httptest.NewRequest(http.MethodPut, path, &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	return rec
}

func (c *chat) avatarRows(t *testing.T) int {
	t.Helper()
	var n int
	require.NoError(t, c.pool.QueryRow(context.Background(), `SELECT count(*) FROM channel_avatars WHERE channel_id = $1`, c.chID).Scan(&n))
	return n
}

func TestChannelAvatarRoutes_UploadServeAndRemove(t *testing.T) {
	c := newChat(t)
	e := c.avatarEcho(true, nil)
	path := "/api/channels/" + c.chID + "/avatar"

	rec := upload(t, e, path, "file", "group.png", pngBytes(t, 200, 120))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	url, _ := decode(t, rec)["avatar_url"].(string)
	require.True(t, strings.HasPrefix(url, "/api/channels/avatars/"), url)

	for _, suffix := range []string{"", "/64", "/192", "/1024"} {
		got := call(e, http.MethodGet, url+suffix, "")
		require.Equal(t, http.StatusOK, got.Code, "GET %s%s", url, suffix)
		assert.Equal(t, "image/jpeg", got.Header().Get("Content-Type"))
		assert.Contains(t, got.Header().Get("Cache-Control"), "immutable")
		assert.Equal(t, "nosniff", got.Header().Get("X-Content-Type-Options"))
	}
	assert.Equal(t, http.StatusNotFound, call(e, http.MethodGet, url+"/100", "").Code)
	assert.Equal(t, http.StatusNotFound, call(e, http.MethodGet, url+"/x", "").Code)
	assert.Equal(t, http.StatusNotFound, call(e, http.MethodGet, "/api/channels/avatars/"+strings.Repeat("0", 32), "").Code)

	rec = call(e, http.MethodDelete, path, "")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Equal(t, "", decode(t, rec)["avatar_url"])
	assert.Equal(t, http.StatusNotFound, call(e, http.MethodGet, url, "").Code, "the old address is gone")
	assert.Zero(t, c.avatarRows(t))
}

func TestChannelAvatarRoutes_DeniedWithoutManage(t *testing.T) {
	c := newChat(t)
	e := c.avatarEcho(false, nil)
	path := "/api/channels/" + c.chID + "/avatar"

	rec := upload(t, e, path, "file", "group.png", pngBytes(t, 50, 50))
	assert.Equal(t, http.StatusForbidden, rec.Code, rec.Body.String())
	assert.Equal(t, http.StatusForbidden, call(e, http.MethodDelete, path, "").Code)
	assert.Zero(t, c.avatarRows(t), "a refused upload stores nothing")
}

func TestChannelAvatarRoutes_UploadRefusals(t *testing.T) {
	c := newChat(t)
	e := c.avatarEcho(true, nil)
	path := "/api/channels/" + c.chID + "/avatar"

	rec := upload(t, e, path, "file", "x.png", []byte("<svg/>"))
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Equal(t, "avatar_not_image", decode(t, rec)["code"])

	rec = upload(t, e, path, "file", "big.png", append(pngBytes(t, 10, 10), make([]byte, domain.AvatarMaxBytes)...))
	assert.Equal(t, http.StatusRequestEntityTooLarge, rec.Code)
	assert.Equal(t, "avatar_too_large", decode(t, rec)["code"])

	rec = upload(t, e, path, "file", "huge.png", pngHeader(10000, 10000, 8, 6))
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Equal(t, "avatar_too_many_pixels", decode(t, rec)["code"])

	rec = upload(t, e, path, "photo", "x.png", pngBytes(t, 10, 10))
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Equal(t, "avatar_missing", decode(t, rec)["code"])

	assert.Equal(t, http.StatusNotFound, upload(t, e, "/api/channels/no-such/avatar", "file", "x.png", pngBytes(t, 10, 10)).Code)
	assert.Zero(t, c.avatarRows(t))
}

// membersPolicy allows every check and puts every node in one workspace's Members
// UA, so two people of a direct message share a workspace as far as the service
// can tell.
type membersPolicy struct {
	answerPolicy
	ws string
}

func (m membersPolicy) GetNode(_ context.Context, r *policypb.GetNodeRequest, _ ...grpc.CallOption) (*policypb.NGACNode, error) {
	return &policypb.NGACNode{Id: r.NodeId, Name: r.NodeId, NodeType: ngac.TypeU}, nil
}

func (m membersPolicy) GetAncestors(context.Context, *policypb.GetAncestorsRequest, ...grpc.CallOption) (*policypb.NodeList, error) {
	name := ngac.MembersUAName(ngac.WorkspaceID(m.ws))
	return &policypb.NodeList{Nodes: []*policypb.NGACNode{{Id: "ua-" + name, Name: name, NodeType: ngac.TypeUA}}}, nil
}

func (m membersPolicy) FindNodeByName(_ context.Context, r *policypb.FindNodeByNameRequest, _ ...grpc.CallOption) (*policypb.NGACNode, error) {
	prefix := "ua-"
	if r.NodeType == ngac.TypePC {
		prefix = "pc-"
	}
	return &policypb.NGACNode{Id: prefix + r.Name, Name: r.Name, NodeType: r.NodeType}, nil
}

func TestChannelAvatarRoutes_DirectMessageIsInvalidInput(t *testing.T) {
	c := newChat(t)
	ctx := context.Background()
	_, err := c.pool.Exec(ctx, `UPDATE channels SET channel_type = 'dm' WHERE id = $1`, c.chID)
	require.NoError(t, err)
	// A direct message has two people; the one writing to it shares a workspace with the other.
	_, peerNode := testutil.CreateUser(t, c.pool)
	_, err = c.pool.Exec(ctx, `INSERT INTO channel_members (channel_id, ngac_node_id) VALUES ($1, $2)`, c.chID, peerNode)
	require.NoError(t, err)
	t.Cleanup(func() { c.pool.Exec(ctx, `DELETE FROM channel_members WHERE channel_id = $1`, c.chID) })
	e := c.avatarEchoWith(membersPolicy{answerPolicy: answerPolicy{allow: true}, ws: c.ws}, nil)

	rec := upload(t, e, "/api/channels/"+c.chID+"/avatar", "file", "x.png", pngBytes(t, 10, 10))

	assert.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
	assert.Zero(t, c.avatarRows(t))
}

func testRedis(t *testing.T) *redis.Client {
	t.Helper()
	addr := os.Getenv("REDIS_ADDR")
	if addr == "" {
		t.Skip("REDIS_ADDR not set")
	}
	rdb := redis.NewClient(&redis.Options{Addr: addr})
	if err := rdb.Ping(context.Background()).Err(); err != nil {
		t.Skipf("redis not available: %v", err)
	}
	t.Cleanup(func() { rdb.Close() })
	return rdb
}

func TestChannelAvatarRoutes_RateLimited(t *testing.T) {
	rdb := testRedis(t)
	c := newChat(t)
	e := c.avatarEcho(true, rdb)
	ctx := context.Background()

	t.Run("uploads per person", func(t *testing.T) {
		key := "channel_avatar_rl:" + c.user
		require.NoError(t, rdb.Set(ctx, key, domain.AvatarUploadLimit, 0).Err())
		t.Cleanup(func() { rdb.Del(ctx, key) })

		rec := upload(t, e, "/api/channels/"+c.chID+"/avatar", "file", "x.png", pngBytes(t, 20, 20))

		assert.Equal(t, http.StatusTooManyRequests, rec.Code, rec.Body.String())
		assert.Equal(t, "rate_limited", decode(t, rec)["code"])
		assert.NotEmpty(t, rec.Header().Get("Retry-After"))
		assert.Zero(t, c.avatarRows(t))
	})

	t.Run("fetches per address", func(t *testing.T) {
		// httptest requests come from 192.0.2.1.
		sum := sha256.Sum256([]byte("192.0.2.1"))
		key := "channel_avatar_pub_rl:" + hex.EncodeToString(sum[:8])
		require.NoError(t, rdb.Set(ctx, key, domain.AvatarPublicLimit, 0).Err())
		t.Cleanup(func() { rdb.Del(ctx, key) })

		rec := call(e, http.MethodGet, "/api/channels/avatars/"+strings.Repeat("0", 32), "")

		assert.Equal(t, http.StatusTooManyRequests, rec.Code)
		assert.NotEmpty(t, rec.Header().Get("Retry-After"))
	})
}

// pngHeader is the signature and IHDR chunk of a PNG of the given size with no
// pixel data: enough for a decoder to read its dimensions.
func pngHeader(w, h int, bitDepth, colorType byte) []byte {
	var ihdr bytes.Buffer
	ihdr.WriteString("IHDR")
	binary.Write(&ihdr, binary.BigEndian, uint32(w))
	binary.Write(&ihdr, binary.BigEndian, uint32(h))
	ihdr.Write([]byte{bitDepth, colorType, 0, 0, 0})
	var out bytes.Buffer
	out.WriteString("\x89PNG\r\n\x1a\n")
	binary.Write(&out, binary.BigEndian, uint32(ihdr.Len()-4))
	out.Write(ihdr.Bytes())
	binary.Write(&out, binary.BigEndian, crc32.ChecksumIEEE(ihdr.Bytes()))
	return out.Bytes()
}
