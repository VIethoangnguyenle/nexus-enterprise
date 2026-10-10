package rest_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"ngac-platform/pkg/httputil"
	pb "ngac-platform/proto/drive"
	"ngac-platform/services/drive/internal/rest"
)

// fakeDrive answers the three reads that carry owners; every other method of
// the interface is unreachable in these tests (nil embedded interface).
type fakeDrive struct {
	rest.DriveService
	list *pb.DriveItemList
	item *pb.DriveItem
}

func (f *fakeDrive) ListFolder(context.Context, *pb.ListFolderRequest) (*pb.DriveItemList, error) {
	return f.list, nil
}
func (f *fakeDrive) GetItem(context.Context, *pb.GetItemRequest) (*pb.DriveItem, error) {
	return f.item, nil
}
func (f *fakeDrive) GetSharedWithMe(context.Context, *pb.GetSharedWithMeRequest) (*pb.DriveItemList, error) {
	return f.list, nil
}

type fakeNames struct {
	names map[string]string
	err   error
	asked []string
}

func (f *fakeNames) DisplayNames(_ context.Context, keys []string) (map[string]string, error) {
	f.asked = keys
	return f.names, f.err
}

func twoOwners() *pb.DriveItemList {
	return &pb.DriveItemList{Items: []*pb.DriveItem{
		{Id: "f1", OwnerId: "user-1", Name: "doi-soat.xlsx"},
		{Id: "d1", OwnerId: "node-2", Name: "Chứng từ gốc"},
		{Id: "d2", OwnerId: "system", Name: "Root"},
		{Id: "f2", OwnerId: "user-1", Name: "sao-ke.pdf"},
	}}
}

func TestOwnerNames_ListGetAndSharedFillOwnerName(t *testing.T) {
	names := &fakeNames{names: map[string]string{"user-1": "Trần Minh Đức", "node-2": "Lê Thị Hoa"}}
	svc := rest.WithOwnerNames(&fakeDrive{list: twoOwners(), item: &pb.DriveItem{OwnerId: "user-1"}}, names)
	ctx := context.Background()

	list, err := svc.ListFolder(ctx, &pb.ListFolderRequest{})
	require.NoError(t, err)
	got := []string{}
	for _, it := range list.Items {
		got = append(got, it.OwnerName)
	}
	assert.Equal(t, []string{"Trần Minh Đức", "Lê Thị Hoa", "", "Trần Minh Đức"}, got,
		"folders keyed by node id and files by user id both resolve; a non-person owner stays blank")
	assert.ElementsMatch(t, []string{"user-1", "node-2", "system"}, names.asked, "one lookup, owners de-duplicated")

	item, err := svc.GetItem(ctx, &pb.GetItemRequest{})
	require.NoError(t, err)
	assert.Equal(t, "Trần Minh Đức", item.OwnerName)

	shared, err := svc.GetSharedWithMe(ctx, &pb.GetSharedWithMeRequest{})
	require.NoError(t, err)
	assert.Equal(t, "Trần Minh Đức", shared.Items[0].OwnerName)
}

// A names lookup that fails must not fail the listing: the screen falls back
// to a neutral label, which is better than an empty folder.
func TestOwnerNames_LookupFailureKeepsTheListing(t *testing.T) {
	names := &fakeNames{err: errors.New("db down")}
	svc := rest.WithOwnerNames(&fakeDrive{list: twoOwners()}, names)

	list, err := svc.ListFolder(context.Background(), &pb.ListFolderRequest{})
	require.NoError(t, err)
	require.Len(t, list.Items, 4)
	for _, it := range list.Items {
		assert.Empty(t, it.OwnerName)
	}
}

func TestOwnerNames_NothingToResolve(t *testing.T) {
	names := &fakeNames{}
	svc := rest.WithOwnerNames(&fakeDrive{list: &pb.DriveItemList{}, item: nil}, names)

	list, err := svc.ListFolder(context.Background(), &pb.ListFolderRequest{})
	require.NoError(t, err)
	assert.Empty(t, list.Items)
	item, err := svc.GetItem(context.Background(), &pb.GetItemRequest{})
	require.NoError(t, err)
	assert.Nil(t, item)
	assert.Nil(t, names.asked, "no owners, no query")
}

// The name reaches the client as an additive `owner_name` field next to the
// existing `owner_id`.
func TestOwnerNames_JSONCarriesOwnerName(t *testing.T) {
	names := &fakeNames{names: map[string]string{"user-1": "Trần Minh Đức"}}
	h := rest.NewHandler(rest.WithOwnerNames(&fakeDrive{list: &pb.DriveItemList{
		Items: []*pb.DriveItem{{Id: "f1", OwnerId: "user-1", Name: "doi-soat.xlsx"}},
	}}, names), nil)

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/workspaces/ws-1/drive", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues("ws-1")
	httputil.SetClaims(c, &httputil.Claims{UserID: "user-9", NGACNodeID: "node-9"})

	require.NoError(t, h.ListRoot(c))
	require.Equal(t, http.StatusOK, rec.Code)

	var body struct {
		Items []map[string]any `json:"items"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.Len(t, body.Items, 1)
	assert.Equal(t, "Trần Minh Đức", body.Items[0]["owner_name"])
	assert.Equal(t, "user-1", body.Items[0]["owner_id"], "existing field is unchanged")
}
