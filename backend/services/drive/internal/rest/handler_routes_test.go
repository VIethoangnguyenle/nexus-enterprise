package rest

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/proto"

	"ngac-platform/ngac"
	"ngac-platform/pkg/httputil"
	pb "ngac-platform/proto/drive"
	policypb "ngac-platform/proto/policy"
)

// spy is the domain as the handlers see it: it keeps the last request it was
// given and answers with err, or with a canned success.
type spy struct {
	DriveService
	last proto.Message
	err  error
}

func (s *spy) take(m proto.Message) error { s.last = m; return s.err }

func (s *spy) CreateFolder(_ context.Context, r *pb.CreateFolderRequest) (*pb.DriveItem, error) {
	return &pb.DriveItem{Id: "i"}, s.take(r)
}
func (s *spy) CreateFile(_ context.Context, r *pb.CreateFileRequest) (*pb.CreateFileResponse, error) {
	return &pb.CreateFileResponse{}, s.take(r)
}
func (s *spy) ConfirmFile(_ context.Context, r *pb.ConfirmFileRequest) (*pb.DriveItem, error) {
	return &pb.DriveItem{Id: "i"}, s.take(r)
}
func (s *spy) GetDownloadURL(_ context.Context, r *pb.GetDownloadURLRequest) (*pb.GetDownloadURLResponse, error) {
	return &pb.GetDownloadURLResponse{}, s.take(r)
}
func (s *spy) RenameItem(_ context.Context, r *pb.RenameItemRequest) (*pb.DriveItem, error) {
	return &pb.DriveItem{Id: "i"}, s.take(r)
}
func (s *spy) MoveItem(_ context.Context, r *pb.MoveItemRequest) (*pb.DriveItem, error) {
	return &pb.DriveItem{Id: "i"}, s.take(r)
}
func (s *spy) CopyItem(_ context.Context, r *pb.CopyItemRequest) (*pb.DriveItem, error) {
	return &pb.DriveItem{Id: "i"}, s.take(r)
}
func (s *spy) TrashItem(_ context.Context, r *pb.TrashItemRequest) (*pb.Empty, error) {
	return &pb.Empty{}, s.take(r)
}
func (s *spy) RestoreItem(_ context.Context, r *pb.RestoreItemRequest) (*pb.DriveItem, error) {
	return &pb.DriveItem{Id: "i"}, s.take(r)
}
func (s *spy) CreateShare(_ context.Context, r *pb.CreateShareRequest) (*pb.ShareInfo, error) {
	return &pb.ShareInfo{}, s.take(r)
}
func (s *spy) RevokeShare(_ context.Context, r *pb.RevokeShareRequest) (*pb.Empty, error) {
	return &pb.Empty{}, s.take(r)
}
func (s *spy) ListShares(_ context.Context, r *pb.ListSharesRequest) (*pb.ShareList, error) {
	return &pb.ShareList{}, s.take(r)
}
func (s *spy) GetSharedWithMe(_ context.Context, r *pb.GetSharedWithMeRequest) (*pb.DriveItemList, error) {
	return &pb.DriveItemList{}, s.take(r)
}
func (s *spy) GetQuota(_ context.Context, r *pb.GetQuotaRequest) (*pb.Quota, error) {
	return &pb.Quota{}, s.take(r)
}

// batchPolicy answers a batch check with a fixed permission table.
type batchPolicy struct {
	policypb.PolicyReadServiceClient
	asked *policypb.BatchCheckAccessRequest
	err   error
}

func (b *batchPolicy) BatchCheckAccess(_ context.Context, r *policypb.BatchCheckAccessRequest, _ ...grpc.CallOption) (*policypb.BatchAccessResult, error) {
	b.asked = r
	if b.err != nil {
		return nil, b.err
	}
	out := map[string]*policypb.ObjectPermissions{}
	for _, id := range r.ObjectIds {
		out[id] = &policypb.ObjectPermissions{Permissions: map[string]bool{ngac.OpRead: true}}
	}
	return &policypb.BatchAccessResult{Results: out}, nil
}

func routes(sp *spy, pol policypb.PolicyReadServiceClient, signedIn bool) *echo.Echo {
	h := NewHandler(sp, pol)
	e := httputil.NewEcho("drive")
	as := func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if signedIn {
				httputil.SetClaims(c, &httputil.Claims{UserID: "u", NGACNodeID: "node-u"})
			}
			return next(c)
		}
	}
	e.POST("/ws/:id/folders", h.CreateFolder, as)
	e.POST("/ws/:id/files", h.CreateFile, as)
	e.POST("/files/:fileId/confirm", h.ConfirmFile, as)
	e.GET("/files/:fileId/download", h.GetDownloadURL, as)
	e.PUT("/items/:itemId/rename", h.RenameItem, as)
	e.POST("/items/:itemId/move", h.MoveItem, as)
	e.POST("/items/:itemId/copy", h.CopyItem, as)
	e.DELETE("/items/:itemId", h.TrashItem, as)
	e.POST("/items/:itemId/restore", h.RestoreItem, as)
	e.POST("/items/:itemId/share", h.CreateShare, as)
	e.DELETE("/shares/:shareId", h.RevokeShare, as)
	e.GET("/items/:itemId/shares", h.ListShares, as)
	e.GET("/shared-with-me", h.SharedWithMe, as)
	e.GET("/ws/:id/quota", h.GetQuota, as)
	e.POST("/batch-access", h.BatchAccess, as)
	return e
}

func send(e *echo.Echo, method, path, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	return rec
}

type route struct {
	name, method, path, body string
	success                  int
	want                     proto.Message
}

func allRoutes() []route {
	return []route{
		{"create folder", "POST", "/ws/w1/folders", `{"name":"Hồ sơ","parent_id":"p1","drive_context":"channel","drive_context_id":"c1"}`, 201,
			&pb.CreateFolderRequest{WorkspaceId: "w1", Name: "Hồ sơ", ParentId: "p1", DriveContext: "channel", DriveContextId: "c1"}},
		{"create file", "POST", "/ws/w1/files", `{"name":"a.pdf","mime_type":"application/pdf","size_bytes":42,"parent_id":"p1"}`, 201,
			&pb.CreateFileRequest{WorkspaceId: "w1", Name: "a.pdf", MimeType: "application/pdf", SizeBytes: 42, ParentId: "p1"}},
		{"confirm", "POST", "/files/f1/confirm", ``, 200, &pb.ConfirmFileRequest{FileId: "f1"}},
		{"download", "GET", "/files/f1/download", ``, 200, &pb.GetDownloadURLRequest{FileId: "f1"}},
		{"rename", "PUT", "/items/i1/rename", `{"name":"new"}`, 200, &pb.RenameItemRequest{ItemId: "i1", NewName: "new"}},
		{"move", "POST", "/items/i1/move", `{"target_folder_id":"t1"}`, 200, &pb.MoveItemRequest{ItemId: "i1", NewParentId: "t1"}},
		{"copy", "POST", "/items/i1/copy", `{"target_folder_id":"t1"}`, 200, &pb.CopyItemRequest{ItemId: "i1", DestParentId: "t1"}},
		{"trash", "DELETE", "/items/i1", ``, 200, &pb.TrashItemRequest{ItemId: "i1"}},
		{"restore", "POST", "/items/i1/restore", ``, 200, &pb.RestoreItemRequest{ItemId: "i1"}},
		{"share", "POST", "/items/i1/share", `{"target_node_id":"n1","share_type":"user","permission":"read"}`, 201,
			&pb.CreateShareRequest{ItemId: "i1", TargetNgacNodeId: "n1", ShareType: "user", Operations: []string{"read"}}},
		{"revoke", "DELETE", "/shares/s1", ``, 200, &pb.RevokeShareRequest{ShareId: "s1"}},
		{"list shares", "GET", "/items/i1/shares", ``, 200, &pb.ListSharesRequest{ItemId: "i1"}},
		{"shared with me", "GET", "/shared-with-me", ``, 200, &pb.GetSharedWithMeRequest{}},
		{"quota", "GET", "/ws/w1/quota", ``, 200, &pb.GetQuotaRequest{WorkspaceId: "w1"}},
	}
}

func TestRoutes_HandOverExactlyWhatTheRequestNamed(t *testing.T) {
	for _, rt := range allRoutes() {
		sp := &spy{}
		rec := send(routes(sp, nil, true), rt.method, rt.path, rt.body)
		require.Equal(t, rt.success, rec.Code, "%s: %s", rt.name, rec.Body)
		assert.True(t, proto.Equal(rt.want, sp.last), "%s: domain was asked %v, want %v", rt.name, sp.last, rt.want)
	}
}

func TestRoutes_RefuseAnonymousCallersBeforeTheDomain(t *testing.T) {
	for _, rt := range allRoutes() {
		sp := &spy{}
		rec := send(routes(sp, nil, false), rt.method, rt.path, rt.body)
		assert.Equal(t, http.StatusUnauthorized, rec.Code, "%s: %s", rt.name, rec.Body)
		assert.Nil(t, sp.last, "%s: the domain was reached without a session", rt.name)
	}
}

func TestRoutes_TurnDomainRefusalsIntoTheirStatus(t *testing.T) {
	for name, tc := range map[string]struct {
		err  error
		want int
	}{
		"denied":   {httputil.ErrAccessDenied, http.StatusForbidden},
		"missing":  {httputil.ErrNotFound, http.StatusNotFound},
		"invalid":  {httputil.ErrInvalidInput, http.StatusBadRequest},
		"internal": {context.DeadlineExceeded, http.StatusInternalServerError},
	} {
		for _, rt := range allRoutes() {
			rec := send(routes(&spy{err: tc.err}, nil, true), rt.method, rt.path, rt.body)
			assert.Equal(t, tc.want, rec.Code, "%s / %s: %s", name, rt.name, rec.Body)
		}
	}
}

func TestRoutes_RefuseMalformedBodies(t *testing.T) {
	for _, rt := range allRoutes() {
		if rt.body == "" {
			continue
		}
		sp := &spy{}
		rec := send(routes(sp, nil, true), rt.method, rt.path, `{`)
		assert.Equal(t, http.StatusBadRequest, rec.Code, "%s: %s", rt.name, rec.Body)
		assert.Nil(t, sp.last, "%s", rt.name)
	}
}

func TestBatchAccess_ChecksTheCallersNodeAndDefaultsTheOperations(t *testing.T) {
	pol := &batchPolicy{}
	e := routes(&spy{}, pol, true)

	rec := send(e, "POST", "/batch-access", `{"object_ids":["a","b"]}`)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Equal(t, "node-u", pol.asked.UserNodeId, "the user asked about is the signed-in one")
	assert.ElementsMatch(t, []string{ngac.OpRead, ngac.OpWrite, ngac.OpShare}, pol.asked.Operations)
	assert.Contains(t, rec.Body.String(), `"a"`)

	pol.asked = nil
	rec = send(e, "POST", "/batch-access", `{"object_ids":[]}`)
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Nil(t, pol.asked, "nothing to ask, nothing asked")

	assert.Equal(t, http.StatusBadRequest, send(e, "POST", "/batch-access", `{`).Code)
}

func TestBatchAccess_APolicyOutageIsNotAnAnswer(t *testing.T) {
	e := routes(&spy{}, &batchPolicy{err: context.DeadlineExceeded}, true)
	rec := send(e, "POST", "/batch-access", `{"object_ids":["a"]}`)
	assert.GreaterOrEqual(t, rec.Code, 500, "an unreachable policy service must not look like a grant: %s", rec.Body)
	assert.NotContains(t, rec.Body.String(), `"read":true`)
}
