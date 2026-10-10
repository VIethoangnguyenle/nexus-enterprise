package rest

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"ngac-platform/pkg/grpcauth"
	"ngac-platform/pkg/httputil"
	pb "ngac-platform/proto/asset"
)

// What the asset screens send, field by field, and what the handlers hand to
// the service. A field the handler does not read is a field the server silently
// ignores, so each request body here is the one the client builds.

type spy struct {
	AssetService
	AssetTypeService
	AssetRequestService

	ctx      context.Context
	listA    *pb.ListAssetsRequest
	create   *pb.CreateAssetRequest
	update   *pb.UpdateAssetRequest
	trans    *pb.TransitionRequest
	handOver *pb.HandOverRequest
	summary  *pb.GetSummaryRequest
	activity *pb.ListActivityRequest
	listR    *pb.ListRequestsReq
	getR     *pb.GetRequestReq
	createR  *pb.CreateAssetRequestReq
	approve  *pb.ApproveRequestReq
	reject   *pb.RejectRequestReq
	assign   *pb.AssignAssetReq
	ret      *pb.ReturnAssetReq
	createT  *pb.CreateTypeRequest
	schema   *pb.UpdateTypeSchemaRequest

	err error
}

func (s *spy) ListAssets(ctx context.Context, r *pb.ListAssetsRequest) (*pb.AssetList, error) {
	s.ctx, s.listA = ctx, r
	return &pb.AssetList{}, s.err
}
func (s *spy) CreateAsset(ctx context.Context, r *pb.CreateAssetRequest) (*pb.Asset, error) {
	s.ctx, s.create = ctx, r
	return &pb.Asset{}, s.err
}
func (s *spy) UpdateAsset(ctx context.Context, r *pb.UpdateAssetRequest) (*pb.Asset, error) {
	s.ctx, s.update = ctx, r
	return &pb.Asset{}, s.err
}
func (s *spy) TransitionAsset(ctx context.Context, r *pb.TransitionRequest) (*pb.Asset, error) {
	s.ctx, s.trans = ctx, r
	return &pb.Asset{}, s.err
}
func (s *spy) HandOverAsset(ctx context.Context, r *pb.HandOverRequest) (*pb.Asset, error) {
	s.ctx, s.handOver = ctx, r
	return &pb.Asset{}, s.err
}
func (s *spy) GetSummary(ctx context.Context, r *pb.GetSummaryRequest) (*pb.AssetSummary, error) {
	s.ctx, s.summary = ctx, r
	return &pb.AssetSummary{}, s.err
}
func (s *spy) ListActivity(ctx context.Context, r *pb.ListActivityRequest) (*pb.ActivityList, error) {
	s.ctx, s.activity = ctx, r
	return &pb.ActivityList{}, s.err
}
func (s *spy) ListRequests(ctx context.Context, r *pb.ListRequestsReq) (*pb.AssetRequestList, error) {
	s.ctx, s.listR = ctx, r
	return &pb.AssetRequestList{}, s.err
}
func (s *spy) GetRequest(ctx context.Context, r *pb.GetRequestReq) (*pb.AssetRequest, error) {
	s.ctx, s.getR = ctx, r
	return &pb.AssetRequest{}, s.err
}
func (s *spy) CreateRequest(ctx context.Context, r *pb.CreateAssetRequestReq) (*pb.AssetRequest, error) {
	s.ctx, s.createR = ctx, r
	return &pb.AssetRequest{}, s.err
}
func (s *spy) ApproveRequest(ctx context.Context, r *pb.ApproveRequestReq) (*pb.AssetRequest, error) {
	s.ctx, s.approve = ctx, r
	return &pb.AssetRequest{}, s.err
}
func (s *spy) RejectRequest(ctx context.Context, r *pb.RejectRequestReq) (*pb.AssetRequest, error) {
	s.ctx, s.reject = ctx, r
	return &pb.AssetRequest{}, s.err
}
func (s *spy) AssignAsset(ctx context.Context, r *pb.AssignAssetReq) (*pb.AssetRequest, error) {
	s.ctx, s.assign = ctx, r
	return &pb.AssetRequest{}, s.err
}
func (s *spy) ReturnAsset(ctx context.Context, r *pb.ReturnAssetReq) (*pb.Empty, error) {
	s.ctx, s.ret = ctx, r
	return &pb.Empty{}, s.err
}
func (s *spy) CreateType(ctx context.Context, r *pb.CreateTypeRequest) (*pb.AssetType, error) {
	s.ctx, s.createT = ctx, r
	return &pb.AssetType{}, s.err
}
func (s *spy) UpdateTypeSchema(ctx context.Context, r *pb.UpdateTypeSchemaRequest) (*pb.AssetType, error) {
	s.ctx, s.schema = ctx, r
	return &pb.AssetType{}, s.err
}

func (s *spy) handler() *Handler { return NewHandler(s, s, s) }

// run calls a handler as the test caller and returns the status it answered.
func run(t *testing.T, h echo.HandlerFunc, method, target, body string, params map[string]string) int {
	t.Helper()
	e := echo.New()
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	var names, values []string
	for k, v := range params {
		names, values = append(names, k), append(values, v)
	}
	c.SetParamNames(names...)
	c.SetParamValues(values...)
	httputil.SetClaims(c, testClaims)
	if err := h(c); err != nil {
		if he, ok := err.(*echo.HTTPError); ok {
			return he.Code
		}
		return http.StatusInternalServerError
	}
	return rec.Code
}

func TestListAssets_ReadsEveryFilterTheScreenSends(t *testing.T) {
	s := &spy{}
	run(t, s.handler().ListAssets, http.MethodGet, "/?type_id=t-1&state=available&search=dell&assigned_to=u-1&limit=6&offset=12", "", map[string]string{"id": "ws-1"})
	require.NotNil(t, s.listA)
	assert.Equal(t, &pb.ListAssetsRequest{WorkspaceId: "ws-1", TypeId: "t-1", State: "available", Search: "dell", AssignedTo: "u-1", Limit: 6, Offset: 12}, s.listA)
	assert.Equal(t, wantCaller, grpcauth.CallerFrom(s.ctx))
}

func TestListAssets_IgnoresNumbersItCannotRead(t *testing.T) {
	s := &spy{}
	assert.Equal(t, http.StatusBadRequest, run(t, s.handler().ListAssets, http.MethodGet, "/?limit=lots", "", map[string]string{"id": "ws-1"}))
	assert.Nil(t, s.listA, "a malformed number is refused before anything is asked of the service")
}

func TestSummaryAndActivity_AskTheServiceAsTheCaller(t *testing.T) {
	s := &spy{}
	run(t, s.handler().GetAssetSummary, http.MethodGet, "/", "", map[string]string{"id": "ws-1"})
	require.NotNil(t, s.summary)
	assert.Equal(t, "ws-1", s.summary.WorkspaceId)
	assert.Equal(t, wantCaller, grpcauth.CallerFrom(s.ctx))

	run(t, s.handler().ListAssetActivity, http.MethodGet, "/?limit=8", "", map[string]string{"id": "ws-1"})
	require.NotNil(t, s.activity)
	assert.Equal(t, &pb.ListActivityRequest{WorkspaceId: "ws-1", Limit: 8}, s.activity)
	assert.Equal(t, wantCaller, grpcauth.CallerFrom(s.ctx))
}

func TestCreateAndUpdateAsset_CarryCustomFields(t *testing.T) {
	s := &spy{}
	run(t, s.handler().CreateAsset, http.MethodPost, "/", `{"type_id":"t-1","name":"Laptop","custom_fields":{"cfg":"M3","ram":18}}`, map[string]string{"id": "ws-1"})
	require.NotNil(t, s.create)
	assert.Equal(t, "t-1", s.create.TypeId)
	assert.Equal(t, "Laptop", s.create.Name)
	assert.Equal(t, "ws-1", s.create.WorkspaceId)
	assert.Equal(t, "M3", s.create.CustomFields.AsMap()["cfg"])
	assert.EqualValues(t, 18, s.create.CustomFields.AsMap()["ram"])

	run(t, s.handler().UpdateAsset, http.MethodPut, "/", `{"name":"Laptop 2","custom_fields":{"cfg":"M4"}}`, map[string]string{"assetId": "a-1"})
	require.NotNil(t, s.update)
	assert.Equal(t, "Laptop 2", s.update.Name)
	assert.Equal(t, "M4", s.update.CustomFields.AsMap()["cfg"])

	// No custom_fields in the body leaves the asset's fields alone (nil, not empty).
	s.update = nil
	run(t, s.handler().UpdateAsset, http.MethodPut, "/", `{"name":"Only name"}`, map[string]string{"assetId": "a-1"})
	assert.Nil(t, s.update.CustomFields)
}

func TestTransitionAsset_TakesTheActionByName(t *testing.T) {
	s := &spy{}
	run(t, s.handler().TransitionAsset, http.MethodPost, "/", `{"action":"flag_maintenance","comment":"Thay pin","user_id":"forged"}`, map[string]string{"assetId": "a-1"})
	require.NotNil(t, s.trans)
	assert.Equal(t, &pb.TransitionRequest{AssetId: "a-1", Action: "flag_maintenance", Comment: "Thay pin"}, s.trans)

	// The older spelling still works.
	run(t, s.handler().TransitionAsset, http.MethodPost, "/", `{"to_state":"retire"}`, map[string]string{"assetId": "a-2"})
	assert.Equal(t, "retire", s.trans.Action)
}

func TestHandOver_NamesThePersonByID(t *testing.T) {
	s := &spy{}
	run(t, s.handler().HandOverAsset, http.MethodPost, "/", `{"assignee_id":"u-9","comment":"Thay máy cũ","user_id":"forged"}`, map[string]string{"assetId": "a-1"})
	require.NotNil(t, s.handOver)
	assert.Equal(t, &pb.HandOverRequest{AssetId: "a-1", AssigneeId: "u-9", Comment: "Thay máy cũ"}, s.handOver)
	assert.Equal(t, wantCaller, grpcauth.CallerFrom(s.ctx))
}

func TestRequests_ListGetAndCreateReadTheFieldsTheScreenSends(t *testing.T) {
	s := &spy{}
	run(t, s.handler().ListAssetRequests, http.MethodGet, "/?status=approved,fulfilled&mine=true&limit=25&offset=50", "", map[string]string{"id": "ws-1"})
	require.NotNil(t, s.listR)
	assert.Equal(t, &pb.ListRequestsReq{WorkspaceId: "ws-1", Status: "approved,fulfilled", MineOnly: true, Limit: 25, Offset: 50}, s.listR)

	run(t, s.handler().GetAssetRequest, http.MethodGet, "/", "", map[string]string{"reqId": "r-1"})
	require.NotNil(t, s.getR)
	assert.Equal(t, "r-1", s.getR.RequestId)
	assert.Equal(t, wantCaller, grpcauth.CallerFrom(s.ctx))

	run(t, s.handler().CreateAssetRequest, http.MethodPost, "/", `{"type_id":"t-1","reason":"Nhân sự mới","urgency":"urgent","quantity":2}`, map[string]string{"id": "ws-1"})
	require.NotNil(t, s.createR)
	assert.Equal(t, &pb.CreateAssetRequestReq{WorkspaceId: "ws-1", TypeId: "t-1", Justification: "Nhân sự mới", Urgency: "urgent", Quantity: 2}, s.createR)
}

func TestApproveRejectAssign_ReadTheirBodies(t *testing.T) {
	s := &spy{}
	run(t, s.handler().ApproveAssetRequest, http.MethodPost, "/", `{"asset_id":"a-4","comment":"Máy mới nhất"}`, map[string]string{"reqId": "r-1"})
	assert.Equal(t, &pb.ApproveRequestReq{RequestId: "r-1", AssetId: "a-4", Comment: "Máy mới nhất"}, s.approve)

	s.approve = nil
	run(t, s.handler().ApproveAssetRequest, http.MethodPost, "/", ``, map[string]string{"reqId": "r-2"})
	assert.Equal(t, &pb.ApproveRequestReq{RequestId: "r-2"}, s.approve, "approving needs no body")

	run(t, s.handler().RejectAssetRequest, http.MethodPost, "/", `{"reason":"Kho còn máy cũ"}`, map[string]string{"reqId": "r-1"})
	assert.Equal(t, &pb.RejectRequestReq{RequestId: "r-1", Reason: "Kho còn máy cũ"}, s.reject)

	run(t, s.handler().AssignAsset, http.MethodPost, "/", `{"asset_id":"a-4"}`, map[string]string{"reqId": "r-1"})
	assert.Equal(t, &pb.AssignAssetReq{RequestId: "r-1", AssetId: "a-4"}, s.assign)
	assert.Equal(t, wantCaller, grpcauth.CallerFrom(s.ctx))

	run(t, s.handler().ReturnAsset, http.MethodPost, "/", ``, map[string]string{"assetId": "a-4"})
	assert.Equal(t, &pb.ReturnAssetReq{AssetId: "a-4"}, s.ret)
}

func TestRejectAsset_AMalformedBodyIsRefusedNotTreatedAsNoReason(t *testing.T) {
	s := &spy{}
	assert.Equal(t, http.StatusBadRequest, run(t, s.handler().RejectAssetRequest, http.MethodPost, "/", `{`, map[string]string{"reqId": "r-1"}))
	assert.Nil(t, s.reject)
}

func TestTypes_CreateAndSchemaReadTheirFields(t *testing.T) {
	s := &spy{}
	run(t, s.handler().CreateAssetType, http.MethodPost, "/", `{"name":"Laptop","category":"hardware","fields_schema":"{\"type\":\"object\"}"}`, map[string]string{"id": "ws-1"})
	require.NotNil(t, s.createT)
	assert.Equal(t, "Laptop", s.createT.Name)
	assert.Equal(t, "hardware", s.createT.Category)
	assert.Equal(t, `{"type":"object"}`, s.createT.FieldsSchema)

	run(t, s.handler().UpdateAssetTypeSchema, http.MethodPut, "/", `{"fields_schema":"{\"type\":\"object\"}"}`, map[string]string{"typeId": "t-1"})
	assert.Equal(t, `{"type":"object"}`, s.schema.FieldsSchema)
}

func TestErrors_MapToTheStatusAScreenCanActOn(t *testing.T) {
	cases := map[codes.Code]int{
		codes.PermissionDenied:   http.StatusForbidden,
		codes.NotFound:           http.StatusNotFound,
		codes.InvalidArgument:    http.StatusBadRequest,
		codes.FailedPrecondition: http.StatusConflict,
		codes.AlreadyExists:      http.StatusConflict,
		codes.Unauthenticated:    http.StatusUnauthorized,
		codes.Internal:           http.StatusInternalServerError,
	}
	for code, want := range cases {
		s := &spy{err: status.Error(code, "no")}
		got := run(t, s.handler().HandOverAsset, http.MethodPost, "/", `{"assignee_id":"u"}`, map[string]string{"assetId": "a"})
		assert.Equal(t, want, got, code.String())
	}
}

func TestInternalErrors_NeverCarryTheDatabaseText(t *testing.T) {
	err := mapGRPCError(status.Error(codes.Internal, `apply transition: ERROR: duplicate key value violates unique constraint "assets_pkey" (SQLSTATE 23505)`))
	assert.Equal(t, http.StatusInternalServerError, err.Code)
	body, _ := json.Marshal(err.Message)
	assert.NotContains(t, string(body), "SQLSTATE")
	assert.NotContains(t, string(body), "assets_pkey")
	assert.Contains(t, string(body), "internal error")

	plain := mapGRPCError(errors.New("pq: connection refused at 10.0.0.5"))
	assert.Equal(t, http.StatusInternalServerError, plain.Code)
	body, _ = json.Marshal(plain.Message)
	assert.NotContains(t, string(body), "10.0.0.5")
}

func TestRefusals_HandTheReasonToTheClient(t *testing.T) {
	st, _ := status.New(codes.FailedPrecondition, "request is not in a state that allows this").
		WithDetails(&errdetails.ErrorInfo{Reason: "request_not_open"})
	err := mapGRPCError(st.Err())
	assert.Equal(t, http.StatusConflict, err.Code)
	body, _ := json.Marshal(err.Message)
	assert.JSONEq(t, `{"message":"request is not in a state that allows this","reason":"request_not_open"}`, string(body))
}
