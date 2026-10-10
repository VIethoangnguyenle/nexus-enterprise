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

	"ngac-platform/pkg/grpcauth"
	"ngac-platform/pkg/httputil"
	pb "ngac-platform/proto/asset"
)

// The guarded RPCs authorize the caller on the context. These tests pin that
// the REST layer puts the authenticated caller from the JWT claims there, and
// that nothing the client sends in the body can replace it.

var wantCaller = grpcauth.Caller{UserID: "user-1", NGACNodeID: "ngac-1"}

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

type captureAssetSvc struct {
	AssetService
	AssetRequestService
	ctx         context.Context
	getAsset    *pb.GetAssetRequest
	transitions *pb.GetTransitionsRequest
	history     *pb.GetHistoryRequest
	createReq   *pb.CreateAssetRequestReq
}

func (c *captureAssetSvc) GetAsset(ctx context.Context, req *pb.GetAssetRequest) (*pb.Asset, error) {
	c.ctx, c.getAsset = ctx, req
	return &pb.Asset{}, nil
}

func (c *captureAssetSvc) GetAvailableTransitions(ctx context.Context, req *pb.GetTransitionsRequest) (*pb.TransitionList, error) {
	c.ctx, c.transitions = ctx, req
	return &pb.TransitionList{}, nil
}

func (c *captureAssetSvc) GetAssetHistory(ctx context.Context, req *pb.GetHistoryRequest) (*pb.TransitionHistoryList, error) {
	c.ctx, c.history = ctx, req
	return &pb.TransitionHistoryList{}, nil
}

func (c *captureAssetSvc) CreateRequest(ctx context.Context, req *pb.CreateAssetRequestReq) (*pb.AssetRequest, error) {
	c.ctx, c.createReq = ctx, req
	return &pb.AssetRequest{}, nil
}

var testClaims = &httputil.Claims{UserID: "user-1", NGACNodeID: "ngac-1"}

func TestGetAsset_PutsCallerOnContext(t *testing.T) {
	svc := &captureAssetSvc{}
	h := NewHandler(svc, nil, svc)
	call(t, http.MethodGet, "", map[string]string{"assetId": "a-1"}, h.GetAsset)
	require.NotNil(t, svc.getAsset)
	assert.Equal(t, wantCaller, grpcauth.CallerFrom(svc.ctx))
}

func TestGetAvailableTransitions_PutsCallerOnContext(t *testing.T) {
	svc := &captureAssetSvc{}
	h := NewHandler(svc, nil, svc)
	call(t, http.MethodGet, "", map[string]string{"assetId": "a-1"}, h.GetAvailableTransitions)
	require.NotNil(t, svc.transitions)
	assert.Equal(t, wantCaller, grpcauth.CallerFrom(svc.ctx))
}

func TestGetAssetHistory_PutsCallerOnContext(t *testing.T) {
	svc := &captureAssetSvc{}
	h := NewHandler(svc, nil, svc)
	call(t, http.MethodGet, "", map[string]string{"assetId": "a-1"}, h.GetAssetHistory)
	require.NotNil(t, svc.history)
	assert.Equal(t, wantCaller, grpcauth.CallerFrom(svc.ctx))
}

// The requester is who the token says, never who the body says.
func TestCreateAssetRequest_TakesRequesterFromClaimsNotBody(t *testing.T) {
	svc := &captureAssetSvc{}
	h := NewHandler(svc, nil, svc)
	body := `{"type_id":"t-1","reason":"need it","quantity":1,
	          "user_id":"forged-user","UserId":"forged-user",
	          "user_ngac_node_id":"forged-node","UserNgacNodeId":"forged-node"}`

	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues("ws-1")
	httputil.SetClaims(c, testClaims)
	require.NoError(t, h.CreateAssetRequest(c))
	assert.Equal(t, http.StatusCreated, rec.Code)

	require.NotNil(t, svc.createReq)
	assert.Equal(t, wantCaller, grpcauth.CallerFrom(svc.ctx))
	assert.Equal(t, "t-1", svc.createReq.TypeId)
}

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

func TestListAssets_PutsCallerOnContext(t *testing.T) {
	svc := &captureSvc{}
	h := NewHandler(svc, svc, nil)
	call(t, http.MethodGet, "", map[string]string{"id": "ws-1"}, h.ListAssets)
	require.NotNil(t, svc.listAssets)
	assert.Equal(t, wantCaller, grpcauth.CallerFrom(svc.ctx))
}

func TestGetAssetSummary_PutsCallerOnContext(t *testing.T) {
	svc := &captureSvc{}
	h := NewHandler(svc, svc, nil)
	call(t, http.MethodGet, "", map[string]string{"id": "ws-1"}, h.GetAssetSummary)
	require.NotNil(t, svc.listAssets)
	assert.Equal(t, wantCaller, grpcauth.CallerFrom(svc.ctx))
}

func TestListAssetTypes_PutsCallerOnContext(t *testing.T) {
	svc := &captureSvc{}
	h := NewHandler(svc, svc, nil)
	call(t, http.MethodGet, "", map[string]string{"id": "ws-1"}, h.ListAssetTypes)
	assert.Equal(t, wantCaller, grpcauth.CallerFrom(svc.ctx))
}

func TestGetAssetType_PutsCallerOnContext(t *testing.T) {
	svc := &captureSvc{}
	h := NewHandler(svc, svc, nil)
	call(t, http.MethodGet, "", map[string]string{"typeId": "t-1"}, h.GetAssetType)
	require.True(t, svc.getTypeSeen)
	assert.Equal(t, wantCaller, grpcauth.CallerFrom(svc.ctx))
}

func TestUpdateAssetTypeSchema_PutsCallerOnContext(t *testing.T) {
	svc := &captureSvc{}
	h := NewHandler(svc, svc, nil)
	call(t, http.MethodPut, `{"fields_schema":"{}"}`, map[string]string{"typeId": "t-1"}, h.UpdateAssetTypeSchema)
	require.NotNil(t, svc.updateType)
	assert.Equal(t, wantCaller, grpcauth.CallerFrom(svc.ctx))
}
