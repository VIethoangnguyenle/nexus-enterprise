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

	"ngac-platform/pkg/httputil"
	pb "ngac-platform/proto/asset"
	"ngac-platform/services/asset/internal/caller"
)

// The guarded RPCs authorize the caller they are handed. These tests pin that
// the REST layer hands them the authenticated caller from the JWT claims —
// in the request field where the message has one, otherwise on the context.

type captureSvc struct {
	AssetService
	AssetTypeService
	ctx         context.Context
	listAssets  *pb.ListAssetsRequest
	updateType  *pb.UpdateTypeSchemaRequest
	getTypeSeen bool
}

func (c *captureSvc) ListAssets(ctx context.Context, req *pb.ListAssetsRequest) (*pb.AssetList, error) {
	c.ctx, c.listAssets = ctx, req
	return &pb.AssetList{}, nil
}

func (c *captureSvc) ListTypes(ctx context.Context, _ *pb.ListTypesRequest) (*pb.AssetTypeList, error) {
	c.ctx = ctx
	return &pb.AssetTypeList{}, nil
}

func (c *captureSvc) GetType(ctx context.Context, _ *pb.GetTypeRequest) (*pb.AssetType, error) {
	c.ctx, c.getTypeSeen = ctx, true
	return &pb.AssetType{}, nil
}

func (c *captureSvc) UpdateTypeSchema(ctx context.Context, req *pb.UpdateTypeSchemaRequest) (*pb.AssetType, error) {
	c.ctx, c.updateType = ctx, req
	return &pb.AssetType{}, nil
}

var testClaims = &httputil.Claims{UserID: "user-1", NGACNodeID: "ngac-1"}

func call(t *testing.T, method, body string, params map[string]string, h echo.HandlerFunc) {
	t.Helper()
	e := echo.New()
	req := httptest.NewRequest(method, "/", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	for k, v := range params {
		c.SetParamNames(k)
		c.SetParamValues(v)
	}
	httputil.SetClaims(c, testClaims)
	require.NoError(t, h(c))
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestListAssets_ForwardsCaller(t *testing.T) {
	svc := &captureSvc{}
	h := NewHandler(svc, svc, nil)
	call(t, http.MethodGet, "", map[string]string{"id": "ws-1"}, h.ListAssets)
	require.NotNil(t, svc.listAssets)
	assert.Equal(t, "ngac-1", svc.listAssets.UserNgacNodeId)
}

func TestGetAssetSummary_ForwardsCaller(t *testing.T) {
	svc := &captureSvc{}
	h := NewHandler(svc, svc, nil)
	call(t, http.MethodGet, "", map[string]string{"id": "ws-1"}, h.GetAssetSummary)
	require.NotNil(t, svc.listAssets)
	assert.Equal(t, "ngac-1", svc.listAssets.UserNgacNodeId)
}

func TestListAssetTypes_PutsCallerOnContext(t *testing.T) {
	svc := &captureSvc{}
	h := NewHandler(svc, svc, nil)
	call(t, http.MethodGet, "", map[string]string{"id": "ws-1"}, h.ListAssetTypes)
	assert.Equal(t, caller.Identity{UserID: "user-1", NGACNodeID: "ngac-1"}, caller.FromContext(svc.ctx))
}

func TestGetAssetType_PutsCallerOnContext(t *testing.T) {
	svc := &captureSvc{}
	h := NewHandler(svc, svc, nil)
	call(t, http.MethodGet, "", map[string]string{"typeId": "t-1"}, h.GetAssetType)
	require.True(t, svc.getTypeSeen)
	assert.Equal(t, caller.Identity{UserID: "user-1", NGACNodeID: "ngac-1"}, caller.FromContext(svc.ctx))
}

func TestUpdateAssetTypeSchema_ForwardsCaller(t *testing.T) {
	svc := &captureSvc{}
	h := NewHandler(svc, svc, nil)
	call(t, http.MethodPut, `{"fields_schema":"{}"}`, map[string]string{"typeId": "t-1"}, h.UpdateAssetTypeSchema)
	require.NotNil(t, svc.updateType)
	assert.Equal(t, "ngac-1", svc.updateType.UserNgacNodeId)
}
